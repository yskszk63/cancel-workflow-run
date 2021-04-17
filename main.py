from __future__ import annotations

import hmac
import logging
import os
import time
from typing import Any, Callable, Generator, Optional
from urllib.parse import urlencode

import jwt
from fastapi import FastAPI, Header, HTTPException, Request, Response
from fastapi.routing import APIRoute
from httpx import AsyncClient
from pydantic import BaseModel

APP_ID = os.environ['APP_ID']
WEBHOOK_SECRET = os.environb[b'WEBHOOK_SECRET']
SECRET = os.environ['SECRET']

logger = logging.getLogger(__name__)
logging.basicConfig(level=logging.DEBUG)


class AppToken(BaseModel):
    token: str
    expires_at: str
    permissions: dict[str, str]
    repository_selection: str


class User(BaseModel):
    login: str


class Repository(BaseModel):
    url: str


class Ref(BaseModel):
    ref: str
    repo: Repository


class Organization(BaseModel):
    pass


class Installation(BaseModel):
    id: int


class PullRequest(BaseModel):
    url: str
    issue_url: str
    user: User
    head: Ref


class Payload(BaseModel):
    action: Optional[str]
    number: Optional[int]
    pull_request: Optional[PullRequest]
    repository: Optional[Repository]
    organization: Optional[Organization]
    installation: Optional[Installation]
    sender: Optional[User]


class PrFile(BaseModel):
    filename: str
    status: str

    @property
    def is_workflow(self: PrFile) -> bool:
        return self.filename.startswith('.github/workflows')

    @property
    def is_added(self: PrFile) -> bool:
        return self.status == 'added'


class WorkflowRun(BaseModel):
    url: str


class VerifySignatureRoute(APIRoute):
    def get_route_handler(self: VerifySignatureRoute) -> Callable:
        original = super().get_route_handler()

        async def custom_route_handler(request: Request) -> Response:
            sign = request.headers.get('X-Hub-Signature-256')
            if sign is None:
                raise HTTPException(422, 'signature not verified')
            body = await request.body()

            digest = hmac.new(WEBHOOK_SECRET,
                              body,
                              'sha256').hexdigest()
            if not hmac.compare_digest(sign, 'sha256=' + digest):
                raise HTTPException(422, 'signature not verified')
            return await original(request)

        return custom_route_handler


async def get_token(client: AsyncClient, installation_id: int) -> AppToken:
    now = int(time.time())

    jwt_payload = {
        'iat': now,
        'exp': now + (60 * 5),
        'iss': APP_ID,
    }

    jwt_token = jwt.encode(jwt_payload, SECRET, 'RS256')

    headers = {
        'Authorization': f'Bearer {jwt_token}',
        'Accept': 'application/vnd.github.v3+json',
    }

    endpoint = 'https://api.github.com'
    url = f'{endpoint}/app/installations/{installation_id}/access_tokens'

    logger.debug(f'retrieve token from {url}')

    response = await client.post(url, headers=headers)
    if response.is_error:
        raise Exception(f'failed to call: {response.status_code} {response.text}')

    return AppToken(**response.json())


async def call_as_api(client: AsyncClient,
                      url: str,
                      method: str,
                      token: AppToken,
                      *,
                      json: Any = None) -> Any:
    headers = {
        'Authorization': f'Bearer {token.token}',
        'Accept': 'application/vnd.github.v3+json',
    }

    logger.debug(f'call: {method} {url}')
    response = await client.request(method, url, headers=headers, json=json)
    if response.is_error:
        raise Exception(f'failed to call: {response.status_code} {response.text}')

    return response.json()


async def list_pr_files(client: AsyncClient,
                        token: AppToken,
                        pull_request: PullRequest) -> list[PrFile]:

    url = f'{pull_request.url}/files'
    response = await call_as_api(client, url, 'get', token)
    return [PrFile(**f) for f in response]


async def comment_pr(client: AsyncClient,
                     token: AppToken,
                     pr: PullRequest,
                     comment: str) -> None:

    url = f'{pr.issue_url}/comments'
    await call_as_api(client, url, 'post', token, json={'body': comment})


async def close_pr(client: AsyncClient,
                   token: AppToken,
                   pr: PullRequest) -> None:

    await call_as_api(client, pr.url, 'post', token, json={'state': 'closed'})


async def list_workflow_runs(client: AsyncClient,
                             token: AppToken,
                             repo: Repository,
                             pr: PullRequest) -> Generator[WorkflowRun,
                                                           None,
                                                           None]:

    params = {
        'actor': pr.user.login,
        'branch': pr.head.ref,
    }
    url = f'{repo.url}/actions/runs?{urlencode(params)}'

    while True:
        json = dict(state='closed')
        response = await call_as_api(client, url, 'get', token, json=json)

        for run in response['workflow_runs']:
            yield WorkflowRun(**run)

        # FIXME
        # https://docs.github.com/ja/rest/guides/traversing-with-pagination
        break


async def cancel_workflow(client: AsyncClient,
                          token: AppToken,
                          run: WorkflowRun) -> None:

    url = f'{run.url}/cancel'
    try:
        await call_as_api(client, url, 'post', token)
    except Exception as err:
        logger.warn(err)


app = FastAPI()
app.router.route_class = VerifySignatureRoute


async def on_ping() -> None:
    pass


async def on_pull_request(payload: Payload) -> None:
    if payload.action == 'closed':
        print('skip')
        return

    pull_request = payload.pull_request
    if pull_request is None:
        raise HTTPException(400)

    repository = payload.repository
    if repository is None:
        raise HTTPException(400)

    async with AsyncClient() as client:
        token = await get_token(client, payload.installation.id)
        files = await list_pr_files(client, token, pull_request)

        workflow_added = [f for f in files if f.is_workflow and f.is_added]
        if len(workflow_added):
            print('detected: workflow added')

            runs = list_workflow_runs(client, token, repository, pull_request)
            async for run in runs:
                await cancel_workflow(client, token, run)

            comment = 'Sorry. Could not accept workflow added.'
            await comment_pr(client, token, pull_request, comment)
            await close_pr(client, token, pull_request)


async def on_installation() -> None:
    pass


@app.post('/webhook',)
async def post(payload: Payload, x_gitHub_event: str = Header(None)) -> Any:
    if x_gitHub_event == 'ping':
        return await on_ping()
    elif x_gitHub_event == 'pull_request':
        return await on_pull_request(payload)
    elif x_gitHub_event == 'installation':
        return await on_installation()
    else:
        raise HTTPException(422)
