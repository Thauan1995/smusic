#!/usr/bin/env python3
"""Minimal path-based reverse proxy for local smusic testing.

Why this exists: cmd/server (REST API) and cmd/presence-server (WebSocket
presence) are deliberately separate processes/ports by design (see
docs/architecture/backend-go.md section 1), but the Flutter clients only
take a single SMUSIC_API_BASE_URL and derive the presence WS URL from it
by swapping http(s) -> ws(s) on the SAME host:port (see
buildPresenceUri in app/smusic_mobile and app/smusic_web's main.dart).
That means a client can only reach both servers if they appear to live
on one port. This proxy provides that single port for local/LAN testing:
requests whose path starts with /v1/presence/ go to the presence server,
everything else goes to the REST API server.

This is dev tooling only - no TLS, minimal error handling, not meant for
production. Standard library only (asyncio), no dependencies.

Usage: path_proxy.py <listen_port> <rest_port> <presence_port> [bind_host]
"""
import asyncio
import sys

PRESENCE_PREFIX = b"/v1/presence/"
CHUNK = 65536


async def _relay(reader: asyncio.StreamReader, writer: asyncio.StreamWriter) -> None:
    try:
        while True:
            data = await reader.read(CHUNK)
            if not data:
                break
            writer.write(data)
            await writer.drain()
    except (ConnectionResetError, BrokenPipeError, asyncio.IncompleteReadError):
        pass
    finally:
        writer.close()


async def _handle(
    client_reader: asyncio.StreamReader,
    client_writer: asyncio.StreamWriter,
    rest_port: int,
    presence_port: int,
) -> None:
    try:
        head = await client_reader.readuntil(b"\r\n\r\n")
    except (asyncio.IncompleteReadError, ConnectionResetError):
        client_writer.close()
        return

    request_line_end = head.find(b"\r\n")
    request_line = head[:request_line_end] if request_line_end != -1 else head
    target_port = presence_port if PRESENCE_PREFIX in request_line else rest_port

    try:
        upstream_reader, upstream_writer = await asyncio.open_connection(
            "127.0.0.1", target_port
        )
    except OSError:
        client_writer.close()
        return

    upstream_writer.write(head)
    await upstream_writer.drain()

    await asyncio.gather(
        _relay(client_reader, upstream_writer),
        _relay(upstream_reader, client_writer),
    )


async def main() -> None:
    if len(sys.argv) < 4:
        print(__doc__)
        sys.exit(1)

    listen_port = int(sys.argv[1])
    rest_port = int(sys.argv[2])
    presence_port = int(sys.argv[3])
    bind_host = sys.argv[4] if len(sys.argv) > 4 else "0.0.0.0"

    async def handler(r: asyncio.StreamReader, w: asyncio.StreamWriter) -> None:
        await _handle(r, w, rest_port, presence_port)

    server = await asyncio.start_server(handler, bind_host, listen_port)
    print(
        f"path_proxy: listening on {bind_host}:{listen_port} -> "
        f"/v1/presence/* to 127.0.0.1:{presence_port}, everything else to "
        f"127.0.0.1:{rest_port}",
        flush=True,
    )
    async with server:
        await server.serve_forever()


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        pass
