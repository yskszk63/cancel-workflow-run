import os
from typing import Optional
import hmac

from fastapi import FastAPI, Header, Request, HTTPException
from fastapi.responses import RedirectResponse
from pydantic import BaseModel
from httpx import AsyncClient
import aiosqlite

CLIENT_ID = os.environ['CLIENT_ID']
CLIENT_SECRET = os.environ['CLIENT_SECRET']
WEBHOOK_SECRET = os.environ['WEBHOOK_SECRET']

database = {}
db = None


class Token(BaseModel):
    access_token: str
    expires_in: int
    refresh_token: str
    refresh_token_expires_in: int
    token_type: str


class Database:
    connection: aiosqlite.Connection

    @classmethod
    async def open(cls, path):
        connection = await aiosqlite.connect(path)

        try:
            await connection.execute("CREATE TABLE IF NOT EXISTS token (id INT, access_token TEXT, expires_in int, refresh_token str, refresh_token_expires_in int, token_type str, primary key (id))")
        except:
            await connection.close()
            raise

        return cls(connection)

    def __init__(self, connection):
        self.connection = connection

    async def close(self):
        await self.connection.close()

    async def put_token(self, installation_id: int, item: Token):
        token = item.dict() | dict(id=installation_id)
        async with self.connection.cursor() as cursor:
            await cursor.execute("UPDATE token SET access_token=:access_token, expires_in=:expires_in, refresh_token=:refresh_token, refresh_token_expires_in=:refresh_token_expires_in, token_type=:token_type WHERE id=:id", token)
            if not cursor.rowcount:
                await cursor.execute("INSERT INTO token (id, access_token, expires_in, refresh_token, refresh_token_expires_in, token_type) VALUES (:id, :access_token, :expires_in, :refresh_token, :refresh_token_expires_in, :token_type)", token)
        await self.connection.commit()

    async def get_token(self, installation_id: int) -> Token:
        async with self.connection.cursor() as cursor:
            await cursor.execute("SELECT id, access_token, expires_in, refresh_token, refresh_token_expires_in, token_type FROM token WHERE id=:id", { 'id': installation_id })
            row = await cursor.fetchone()
            if row is not None:
                id, access_token, expires_in, refresh_token, refresh_token_expires_in, token_type = row
                return Token(id=id, access_token=access_token, expires_in=expires_in, refresh_token=refresh_token, refresh_token_expires_in=refresh_token_expires_in, token_type=token_type)
        raise Exception("not found.")


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


@app.on_event("startup")
async def on_startup():
    global db
    db = await Database.open(".db")

@app.on_event("shutdown")
async def on_shutdown():
    await db.close()

# ?code=9e7991b2cf5366cd5e93&installation_id=16228562&setup_action=install
@app.get("/oauth2/callback")
async def oauth2_callback(code: str, installation_id: int, setup_action: str):
    async with AsyncClient() as client:
        data = {
            'client_id': CLIENT_ID,
            'client_secret': CLIENT_SECRET,
            'code': code,
        }
        headers = {
            'Accept': 'application/json',
        }
        response = await client.post('https://github.com/login/oauth/access_token', data=data, headers=headers)
        if response.is_error:
            raise HTTPException(response.status_code, response.text)

        token = response.json()
        token = Token(**token)
        await db.put_token(installation_id, token)

    return RedirectResponse(f"https://github.com/settings/installations/{installation_id}")

async def on_ping():
    pass

async def on_pull_request(payload: Payload):
    if payload.action == 'closed':
        print("skip")
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
        response = await client.post('https://github.com/login/oauth/access_token', data=data, headers=headers)
        if response.is_error:
            raise HTTPException(response.status_code, response.text)
        token = Token(**response.json())
        await db.put_token(installation_id, token)

        files_response = await client.get(f'{payload.pull_request.url}/files')
        if files_response.is_error:
            raise HTTPException(files_response.status_code, files_response.text)

        files = files_response.json()
        workflow_added = (f for f in files if f['filename'].startswith(".github/workflows/") and f['status'] == 'added')
        if any(workflow_added):
            print("detected: workflow added")

            url = f'{payload.pull_request.head.repo.url}/actions/runs?actor={payload.pull_request.user.login}&branch={payload.pull_request.head.ref}'
            headers = {
                'Authorization': f'Bearer {token.access_token}',
                'Accept': 'application/vnd.github.v3+json',
            }
            workflows = await client.get(url, headers=headers)
            if workflows.is_error:
                raise HTTPException(workflows.status_code, workflows.text)
            urls = (x['url'] for x in workflows.json()['workflow_runs']) # FIXME
            for url in urls:
                response = await client.post(f'{url}/cancel', headers=headers)
                if response.is_error:
                    print(response.status_code, response.text)

            comment = 'Sorry. Could not accept workflow added.'
            comment_response = await client.post(f'{payload.pull_request.issue_url}/comments', headers=headers, json={'body': comment})
            if comment_response.is_error:
                raise HTTPException(comment_response.status_code, comment_response.text)

            close_response = await client.patch(f'{payload.pull_request.url}', headers=headers, json={'state': 'closed'})
            if close_response.is_error:
                raise HTTPException(close_response.status_code, close_response.text)


async def on_installation(payload: Payload):
    installation_id = payload.installation.id
    database[installation_id] = {}

def verify_payload(payload: bytes, sig: str):
    digest = hmac.new(WEBHOOK_SECRET.encode('utf8'), payload, 'sha256').hexdigest()
    return hmac.compare_digest(sig, 'sha256=' + digest)

@app.post("/webhook")
async def post(request: Request, payload: Payload, x_gitHub_event: str = Header(None), x_hub_signature_256: str = Header(None)):
    if not verify_payload(await request.body(), x_hub_signature_256):
        raise HTTPException(422, "signature not verified")

    if x_gitHub_event == 'ping':
        return await on_ping()
    elif x_gitHub_event == 'pull_request':
        return await on_pull_request(payload)
    elif x_gitHub_event == 'installation':
        return await on_installation(payload)
    else:
        raise HTTPException(422)
