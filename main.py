from __future__ import annotations

import hmac
import os
import time
from typing import Any, Optional
from urllib.parse import urlencode

from fastapi import FastAPI, Header, HTTPException, Request
from httpx import AsyncClient
import jwt
from pydantic import BaseModel

APP_ID = os.environ['APP_ID']
WEBHOOK_SECRET = os.environ['WEBHOOK_SECRET']
SECRET = os.environ['SECRET']


# {'token': 'ghs_1UvhwD3fN3oqRJBhgKnGXOARUxbuwo32iQEa', 'expires_at': '2021-04-16T12:22:37Z', 'permissions': {'actions': 'write', 'metadata': 'read', 'pull_requests': 'write'}, 'repository_selection': 'selected'}
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


app = FastAPI()


async def on_ping() -> None:
    pass


async def on_pull_request(payload: Payload) -> None:
    if payload.action == 'closed':
        print('skip')
        return

    now = int(time.time())
    jwt_payload = {
        'iat': now,
        'exp': now + (60 * 5),
        'iss': APP_ID,
    }
    jwt_token = jwt.encode(jwt_payload, SECRET, 'RS256')

    async with AsyncClient() as client:
        headers = {
            'Authorization': f'Bearer {jwt_token}',
            'Accept': 'application/vnd.github.v3+json',
        }
        url = f'https://api.github.com/app/installations/{payload.installation.id}/access_tokens'
        token_response = await client.post(url, headers=headers)
        if token_response.is_error:
            raise HTTPException(token_response.status_code,
                                token_response.text)
        token = AppToken(**token_response.json())

        files_response = await client.get(f'{payload.pull_request.url}/files')
        if files_response.is_error:
            raise HTTPException(files_response.status_code,
                                files_response.text)

        files = files_response.json()

        def file_added(f: dict[str, Any]) -> bool:
            return (f['filename'].startswith('.github/workflows')
                    and f['status'] == 'added')

        workflow_added = (f for f in files if file_added(f))
        if any(workflow_added):
            print('detected: workflow added')

            query = {
                'actor': payload.pull_request.user.login,
                'branch': payload.pull_request.head.ref,
            }
            url = payload.repository.url
            url = f'{url}/actions/runs?{urlencode(query)}'
            headers = {
                'Authorization': f'Bearer {token.token}',
                'Accept': 'application/vnd.github.v3+json',
            }
            workflows = await client.get(url, headers=headers)
            if workflows.is_error:
                raise HTTPException(workflows.status_code, workflows.text)
            # FIXME filter status
            urls = (x['url'] for x in workflows.json()['workflow_runs'])
            for url in urls:
                response = await client.post(f'{url}/cancel', headers=headers)
                if response.is_error:
                    print(response.status_code, response.text)

            comment = 'Sorry. Could not accept workflow added.'
            comment_response = await client.post(
                    f'{payload.pull_request.issue_url}/comments',
                    headers=headers,
                    json={'body': comment})
            if comment_response.is_error:
                raise HTTPException(comment_response.status_code,
                                    comment_response.text)

            close_response = await client.patch(f'{payload.pull_request.url}',
                                                headers=headers,
                                                json={'state': 'closed'})
            if close_response.is_error:
                raise HTTPException(close_response.status_code,
                                    close_response.text)


async def on_installation() -> None:
    pass


def verify_payload(payload: bytes, sig: str) -> bool:
    digest = hmac.new(WEBHOOK_SECRET.encode('utf8'),
                      payload,
                      'sha256').hexdigest()
    return hmac.compare_digest(sig, 'sha256=' + digest)


@app.post('/webhook')
async def post(request: Request,
               payload: Payload,
               x_gitHub_event: str = Header(None),
               x_hub_signature_256: str = Header(None)) -> Any:

    if not verify_payload(await request.body(), x_hub_signature_256):
        raise HTTPException(422, 'signature not verified')

    if x_gitHub_event == 'ping':
        return await on_ping()
    elif x_gitHub_event == 'pull_request':
        return await on_pull_request(payload)
    elif x_gitHub_event == 'installation':
        return await on_installation()
    else:
        raise HTTPException(422)
