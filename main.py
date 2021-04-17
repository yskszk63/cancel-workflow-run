from __future__ import annotations

import asyncio
import hmac
import logging
import os
import re
import time
from typing import Any, AsyncGenerator, Callable, Optional
from urllib.parse import urlencode

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

# <https://api.github.com/search/code?q=addClass+user%3Amozilla&page=2>; rel="next", ...
link_header_pattern = re.compile(r'^<(?P<url>[^>]*)>(?:\s*;\s*(?P<attrs>(?:[^=]*="[^"]*"(?:\s*;\s*[^=]*="[^"]*")*)))?(?:\s*,\s*|$)')
# rel="next"; ..
attr_pattern = re.compile(r'^(?P<name>[^=]*)="(?P<val>[^"]*)"(?:\s*;\s*|$)')


logger = logging.getLogger(__name__)
logging.basicConfig(level=logging.DEBUG)


class Link(BaseModel):
    url: str
    attrs: dict[str, str]


def iter_attr(target):
    while len(target):
        match = attr_pattern.match(target)
        if not match:
            return

        yield match.group('name'), match.group('val')
        target = target[match.end():]


def iter_links(target):
    while len(target):
        match = link_header_pattern.match(target)
        if not match:
            return

        attrs = dict(iter_attr(match.group('attrs')))
        yield Link(url=match.group('url'), attrs=attrs)
        target = target[match.end():]


def link_header_by_rel(target):
    return { l.attrs['rel']: l for l in iter_links(target) if 'rel' in l.attrs }


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


class Ref(BaseModel):
    ref: str
    repo: Repository
    sha: str


class Organization(BaseModel):
    pass


class Installation(BaseModel):
    id: int


class PullRequest(BaseModel):
    number: int
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
    head_sha: str
    status: str


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


async def iter_as_app(client: AsyncClient,
                      url: str,
                      token: AppToken,
                      key: str) -> AsyncGenerator[Any, None]:

    headers = {
        'Authorization': f'Bearer {token.token}',
        'Accept': 'application/vnd.github.v3+json',
    }

    if url.startswith('/'):
        url = f'{GITHUB_ENDPOINT}{url}'
    logger.debug(f'call: get {url}')
    while True:
        response = await client.get(url, headers=headers)
        try:
            response.raise_for_status()
        except HTTPStatusError:
            logger.warning(response.text)
            raise

        for item in response.json()[key]:
            yield item

        try:
            links = response.headers['link']
            url = link_header_by_rel(links)['next'].url
        except KeyError:
            return


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


async def close_pr(client: AsyncClient,
                   token: AppToken,
                   pr: PullRequest) -> None:

    await call_as_app(client, pr.url, 'post', token, json={'state': 'closed'})


async def iter_workflow_runs(client: AsyncClient,
                             token: AppToken,
                             repo: Repository,
                             pr: PullRequest) -> AsyncGenerator[WorkflowRun, None]:

    params = {
        'actor': pr.user.login,
        'branch': pr.head.ref,
    }
    url = f'{repo.url}/actions/runs?{urlencode(params)}'

    async for run in iter_as_app(client, url, token, 'workflow_runs'):
        yield WorkflowRun(**run)


async def cancel_workflow(client: AsyncClient,
                          token: AppToken,
                          run: WorkflowRun) -> None:

    url = f'{run.url}/cancel'
    try:
        await call_as_app(client, url, 'post', token)
    except Exception as err:
        logger.warning(err)


app = FastAPI()
app.router.route_class = VerifySignatureRoute


async def on_ping() -> None:
    pass


async def on_pull_request(payload: Payload) -> JSONResponse:
    if payload.action == 'closed':
        print('skip')
        return JSONResponse(status_code=200)

    pull_request = payload.pull_request
    if pull_request is None:
        raise HTTPException(400)

    repository = payload.repository
    if repository is None:
        raise HTTPException(400)

    asyncio.create_task(reject_pr(payload.installation.id, repository.full_name, pull_request.number))
    return JSONResponse(status_code=201)


async def on_installation() -> None:
    pass


@app.post('/webhook')
async def post(payload: Payload, x_gitHub_event: str = Header(None)) -> Any:
    if x_gitHub_event == 'ping':
        return await on_ping()
    elif x_gitHub_event == 'pull_request':
        return await on_pull_request(payload)
    elif x_gitHub_event == 'installation':
        return await on_installation()
    else:
        raise HTTPException(422)


async def reject_pr(installation_id: int, repo_name: str, pr_num: int) -> None:
    async with AsyncClient() as client:
        token = await get_token(client, installation_id)

        repo = await get_repository(client, token, repo_name)
        pr = await get_pr(client, token, repo_name, pr_num)

        change_files = await list_pr_files(client, token, pr)
        workflow_added = [f for f in change_files if f.is_workflow and f.is_added]
        if len(workflow_added):
            logger.debug('detected: workflow added')

            async for run in iter_workflow_runs(client, token, repo, pr):
                if run.head_sha == pr.head.sha and run.status != 'completed':
                    await cancel_workflow(client, token, run)

            comment = 'Sorry. Could not accept workflow added.'
            await comment_pr(client, token, pr, comment)
            await close_pr(client, token, pr)


if __name__ == '__main__':
    import sys

    installation_id, repository, pr_num = sys.argv[1:]
    asyncio.run(reject_pr(int(installation_id), repository, int(pr_num)))
