from __future__ import annotations

import hmac
import os
from typing import Any, Optional
from urllib.parse import urlencode

import asyncpg
from fastapi import FastAPI, Header, HTTPException, Request
from fastapi.responses import RedirectResponse
from httpx import AsyncClient
from pydantic import BaseModel

CLIENT_ID = os.environ['CLIENT_ID']
CLIENT_SECRET = os.environ['CLIENT_SECRET']
WEBHOOK_SECRET = os.environ['WEBHOOK_SECRET']
DATABASE_URL = os.environ['DATABASE_URL']
TOKEN_ENDPOINT = os.environ.get('TOKEN_ENDPOINT',
                                'https://github.com/login/oauth/access_token')

db = None


class Token(BaseModel):
    access_token: str
    expires_in: int
    refresh_token: str
    refresh_token_expires_in: int
    token_type: str


class Database:
    pool: asyncpg.Pool

    @classmethod
    async def open(cls: type[Database], url: str) -> Database:
        pool = await asyncpg.create_pool(url, min_size=1, max_size=1)
        if pool is None:
            raise Exception('failed to get connection pool.')

        try:
            sql = """
                CREATE TABLE IF NOT EXISTS token (
                    id INT NOT NULL,
                    access_token TEXT NOT NULL,
                    expires_in INT NOT NULL,
                    refresh_token TEXT NOT NULL,
                    refresh_token_expires_in TEXT NOT NULL,
                    token_type TEXT NOT NULL,
                    primary key (id)
                )
            """
            await pool.execute(sql)

        except Exception:
            await pool.close()
            raise

        return cls(pool)

    def __init__(self: Database, pool: asyncpg.Pool) -> None:
        self.pool = pool

    async def close(self: Database) -> None:
        await self.pool.close()

    async def put_token(self: Database,
                        installation_id: int,
                        token: Token) -> None:

        async with self.pool.acquire() as connection:
            connection: asyncpg.Connection

            async with connection.transaction():
                sql = """
                    INSERT INTO token (
                        id,
                        access_token,
                        expires_in,
                        refresh_token,
                        refresh_token_expires_in,
                        token_type
                    ) VALUES ($1, $2, $3, $4, $5, $6)
                    ON CONFLICT ON CONSTRAINT token_pkey DO
                    UPDATE SET
                        access_token = $2,
                        expires_in = $3,
                        refresh_token = $4,
                        refresh_token_expires_in = $5,
                        token_type = $6
                """

                await connection.execute(sql,
                                         installation_id,
                                         token.access_token,
                                         token.expires_in,
                                         token.refresh_token,
                                         token.refresh_token_expires_in,
                                         token.token_type)

    async def get_token(self: Database, installation_id: int) -> Token:
        async with self.pool.acquire() as connection:
            connection: asyncpg.Connection

            sql = """
                SELECT
                    id,
                    access_token,
                    expires_in,
                    refresh_token,
                    refresh_token_expires_in,
                    token_type
                FROM
                    token
                WHERE
                    id = $1
            """

            row = await connection.fetchrow(sql, installation_id)
            if row is not None:
                return Token(**row)
        raise Exception('not found.')


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
    sender: User


app = FastAPI()


@app.on_event('startup')
async def on_startup() -> None:
    global db
    db = await Database.open(DATABASE_URL)


@app.on_event('shutdown')
async def on_shutdown() -> None:
    await db.close()


# ?code=xxxxx&installation_id=xxxx&setup_action=install
@app.get('/oauth2/callback')
async def oauth2_callback(code: str, installation_id: int) -> RedirectResponse:
    async with AsyncClient() as client:
        data = {
            'client_id': CLIENT_ID,
            'client_secret': CLIENT_SECRET,
            'code': code,
        }
        headers = {
            'Accept': 'application/json',
        }
        response = await client.post(TOKEN_ENDPOINT,
                                     data=data,
                                     headers=headers)
        if response.is_error:
            raise HTTPException(response.status_code, response.text)

        token = response.json()
        token = Token(**token)
        await db.put_token(installation_id, token)

    return RedirectResponse(
            f'https://github.com/settings/installations/{installation_id}')


async def on_ping() -> None:
    pass


async def on_pull_request(payload: Payload) -> None:
    if payload.action == 'closed':
        print('skip')
        return

    installation_id = payload.installation.id
    token = await db.get_token(installation_id)
    refresh_token = token.refresh_token

    async with AsyncClient() as client:
        data = {
            'refresh_token': refresh_token,
            'grant_type': 'refresh_token',
            'client_id': CLIENT_ID,
            'client_secret': CLIENT_SECRET,
        }
        headers = {
            'Accept': 'application/json',
        }
        response = await client.post(TOKEN_ENDPOINT,
                                     data=data,
                                     headers=headers)
        if response.is_error:
            raise HTTPException(response.status_code, response.text)
        token = Token(**response.json())
        await db.put_token(installation_id, token)

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
                'Authorization': f'Bearer {token.access_token}',
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
