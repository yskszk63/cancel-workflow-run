from __future__ import annotations

import asyncio
import hmac
import logging
import os
import time
from typing import Any, Callable, Optional

import jwt
from fastapi import FastAPI, Header, HTTPException, Request, Response
from fastapi.responses import JSONResponse
from fastapi.routing import APIRoute
from httpx import AsyncClient, HTTPStatusError
from pydantic import BaseModel

APP_ID = os.environ['APP_ID']
WEBHOOK_SECRET = os.environb[b'WEBHOOK_SECRET']
SECRET = os.environ['SECRET']
GITHUB_ENDPOINT = os.environ.get('GITHUB_ENDPOINT',
                                 'https://api.github.com')

logger = logging.getLogger(__name__)
logging.basicConfig(level=logging.DEBUG)


comment_template = '''@{opener} @{owner}
Hi, I'm a bot.

Sorry, [This workflow run]({run_url}) is canceled.
Because currently could not accept added at pull request.
'''


class AppToken(BaseModel):
    token: str
    expires_at: str
    permissions: dict[str, str]
    repository_selection: str


class User(BaseModel):
    login: str


class Repository(BaseModel):
    full_name: str
    url: str
    owner: User


class Installation(BaseModel):
    id: int


class PullRequest(BaseModel):
    url: str
    issue_url: str
    user: User


class PrFile(BaseModel):
    filename: str
    status: str

    @property
    def is_added(self: PrFile) -> bool:
        return self.status == 'added'


class WorkflowRunPr(BaseModel):
    number: int


class WorkflowRun(BaseModel):
    url: str
    html_url: str
    workflow_url: str
    id: int
    pull_requests: list[WorkflowRunPr]


class Workflow(BaseModel):
    path: str


class Payload(BaseModel):
    action: Optional[str]
    workflow_run: Optional[WorkflowRun]
    repository: Optional[Repository]
    installation: Optional[Installation]


class VerifySignatureRoute(APIRoute):
    def get_route_handler(self) -> Callable:
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
    try:
        response.raise_for_status()
    except HTTPStatusError:
        logger.warning(response.text)
        raise

    return AppToken(**response.json())


async def call_as_app(client: AsyncClient,
                      url: str,
                      method: str,
                      token: AppToken,
                      *,
                      json: Any = None) -> Any:
    headers = {
        'Authorization': f'Bearer {token.token}',
        'Accept': 'application/vnd.github.v3+json',
    }

    if url.startswith('/'):
        url = f'{GITHUB_ENDPOINT}{url}'
    logger.debug(f'call: {method} {url}')
    response = await client.request(method, url, headers=headers, json=json)
    try:
        response.raise_for_status()
    except HTTPStatusError:
        logger.warning(response.text)
        raise
    return response.json()


async def get_repository(client: AsyncClient,
                         token: AppToken,
                         name: str) -> Any:

    response = await call_as_app(client, f'/repos/{name}', 'get', token)
    return Repository(**response)


async def get_pr(client: AsyncClient,
                 token: AppToken,
                 repo_name: str,
                 pr_num: int) -> PullRequest:

    response = await call_as_app(client, f'/repos/{repo_name}/pulls/{pr_num}', 'get', token)
    return PullRequest(**response)


async def list_pr_files(client: AsyncClient,
                        token: AppToken,
                        pull_request: PullRequest) -> list[PrFile]:

    url = f'{pull_request.url}/files'
    response = await call_as_app(client, url, 'get', token)
    return [PrFile(**f) for f in response]


async def comment_pr(client: AsyncClient,
                     token: AppToken,
                     pr: PullRequest,
                     comment: str) -> None:

    url = f'{pr.issue_url}/comments'
    await call_as_app(client, url, 'post', token, json={'body': comment})


async def get_workflow_run(client: AsyncClient,
                           token: AppToken,
                           repo: Repository,
                           run_id: int) -> WorkflowRun:
    url = f'{repo.url}/actions/runs/{run_id}'
    result = await call_as_app(client, url, 'get', token)
    return WorkflowRun(**result)


async def cancel_workflow(client: AsyncClient,
                          token: AppToken,
                          run: WorkflowRun) -> None:

    url = f'{run.url}/cancel'
    try:
        await call_as_app(client, url, 'post', token)
    except Exception as err:
        logger.warning(err)


async def get_workflow_for_run(client: AsyncClient,
                               token: AppToken,
                               run: WorkflowRun) -> Workflow:
    url = run.workflow_url
    result = await call_as_app(client, url, 'get', token)
    return Workflow(**result)


app = FastAPI()
app.router.route_class = VerifySignatureRoute


async def on_ping() -> None:
    pass


async def on_workflow_run(payload: Payload) -> JSONResponse:
    if payload.action == 'completed':
        logger.debug('skip')
        return JSONResponse(status_code=200)

    repository = payload.repository
    if repository is None:
        raise HTTPException(400)

    workflow_run = payload.workflow_run
    if workflow_run is None:
        raise HTTPException(400)

    pr_nums = [r.number for r in workflow_run.pull_requests]
    asyncio.create_task(cancel_run(payload.installation.id, repository.full_name, workflow_run.id, pr_nums))
    return JSONResponse(status_code=201)


async def on_installation() -> None:
    pass


@app.post('/webhook')
async def post(payload: Payload, x_gitHub_event: str = Header(None)) -> Any:
    if x_gitHub_event == 'ping':
        return await on_ping()
    elif x_gitHub_event == 'workflow_run':
        return await on_workflow_run(payload)
    elif x_gitHub_event == 'installation':
        return await on_installation()
    else:
        raise HTTPException(422)


async def cancel_run(installation_id: int, repo_name: str, run_id: int, pr_nums: list[int]) -> None:
    async with AsyncClient() as client:
        token = await get_token(client, installation_id)

        repo = await get_repository(client, token, repo_name)
        run = await get_workflow_run(client, token, repo, run_id)
        workflow = await get_workflow_for_run(client, token, run)
        for pr_num in pr_nums:
            pr = await get_pr(client, token, repo_name, pr_num)
            for pr_file in await list_pr_files(client, token, pr):
                if pr_file.filename == workflow.path and pr_file.is_added:
                    await cancel_workflow(client, token, run)

                    comment = comment_template.format(run_url=run.html_url,
                                                      opener=pr.user.login,
                                                      owner=repo.owner.login)
                    await comment_pr(client, token, pr, comment)


if __name__ == '__main__':
    import sys

    installation_id, repository, run_id, *pr_nums = sys.argv[1:]
    asyncio.run(cancel_run(int(installation_id), repository, int(run_id), [int(i) for i in pr_nums]))
