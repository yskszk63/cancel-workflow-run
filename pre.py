from __future__ import annotations

import asyncio
import os

import asyncpg

DATABASE_URL = os.environ['DATABASE_URL']


async def main() -> None:
    connection = await asyncpg.connect(DATABASE_URL)
    try:
        sql = """
            CREATE TABLE IF NOT EXISTS token (
                id INT NOT NULL,
                access_token BYTEA NOT NULL,
                expires_in INT NOT NULL,
                refresh_token BYTEA NOT NULL,
                refresh_token_expires_in INT NOT NULL,
                token_type TEXT NOT NULL,
                primary key (id)
            )
        """
        await connection.execute(sql)

        sql = 'CREATE EXTENSION IF NOT EXISTS pgcrypto'
        await connection.execute(sql)

    finally:
        await connection.close()
    print("SUCCESS")

asyncio.run(main())
