#!/usr/bin/env python3
"""
Python Connect RPC Worker for IDA Headless Analysis

IPC transport is chosen automatically based on the arguments:
  --socket PATH   Unix domain socket (Linux / macOS)
  --port  PORT    TCP loopback on 127.0.0.1 (Windows, or when explicitly requested)
"""

import argparse
import logging
import os
import socket
import sys
import types
from pathlib import Path

# ── Protobuf generated code lives in gen/ ────────────────────────────────────
# Pre-register ida.worker.* into sys.modules BEFORE importing the IDA Pro 'ida'
# package.  Without this, 'import ida as idapro' fills sys.modules['ida'] with
# the IDA Pro package, and subsequent 'from ida.worker.v1 import ...' fails
# because the IDA Pro 'ida' package has no 'worker' sub-package.
_gen_dir = Path(__file__).parent / "gen"
sys.path.insert(0, str(_gen_dir))

for _mod_name, _rel in [
    ("ida.worker",    "ida/worker"),
    ("ida.worker.v1", "ida/worker/v1"),
]:
    _m = types.ModuleType(_mod_name)
    _m.__path__ = [str(_gen_dir / _rel)]
    _m.__package__ = _mod_name
    sys.modules.setdefault(_mod_name, _m)

try:
    import ida as idapro  # IDA Pro 9.0 idalib: package is 'ida', alias as 'idapro' for compat
except ImportError:
    print("Error: ida (idalib) module not found. Run py-activate-idalib.py from your IDA Pro installation.")
    sys.exit(1)

from connect_server import ConnectServer
from ida_wrapper import IDAWrapper


def serve(server_socket: socket.socket, handler, session_id: str, label: str):
    """Accept connections on *server_socket* and dispatch each one to *handler*."""
    logging.info(f"[Worker {session_id}] Listening on {label}")
    try:
        while True:
            conn, _ = server_socket.accept()
            handle_connection(conn, handler)
    finally:
        server_socket.close()


def make_unix_socket(path: str) -> socket.socket:
    """Create and bind a Unix domain socket at *path*."""
    if os.path.exists(path):
        os.remove(path)
    sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    sock.bind(path)
    sock.listen(5)
    return sock


def make_tcp_socket(port: int) -> socket.socket:
    """Create and bind a TCP loopback socket on *port*."""
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    sock.bind(("127.0.0.1", port))
    sock.listen(5)
    return sock


def handle_connection(conn: socket.socket, handler):
    """Handle a single HTTP/1.1 request arriving on *conn*."""
    try:
        request_data = b""
        while True:
            chunk = conn.recv(4096)
            if not chunk:
                break
            request_data += chunk
            if b"\r\n\r\n" in request_data:
                headers = request_data.split(b"\r\n\r\n")[0]
                if b"Content-Length:" in headers:
                    for line in headers.split(b"\r\n"):
                        if line.startswith(b"Content-Length:"):
                            content_length = int(line.split(b":")[1].strip())
                            body_start = request_data.find(b"\r\n\r\n") + 4
                            body_received = len(request_data) - body_start
                            if body_received < content_length:
                                remaining = content_length - body_received
                                request_data += conn.recv(remaining)
                break

        if not request_data:
            return

        lines = request_data.split(b"\r\n")
        request_line = lines[0].decode("utf-8")
        method, path, _ = request_line.split()
        response = handler(method, path, request_data)
        conn.sendall(response.encode() if isinstance(response, str) else response)

    except Exception as e:
        logging.error(f"Connection error: {e}")
    finally:
        conn.close()


def main():
    parser = argparse.ArgumentParser(description="IDA Connect Worker")
    # IPC transport — exactly one of --socket or --port must be provided
    transport = parser.add_mutually_exclusive_group(required=True)
    transport.add_argument("--socket", help="Unix domain socket path (Linux/macOS)")
    transport.add_argument("--port", type=int, help="TCP loopback port (Windows)")

    parser.add_argument("--binary", required=True, help="Binary file path")
    parser.add_argument("--session-id", required=True, help="Session ID")
    parser.add_argument("--log-level", default="INFO", help="Log level")
    args = parser.parse_args()

    logging.basicConfig(
        level=getattr(logging, args.log_level),
        format=f"[Worker {args.session_id}] %(asctime)s - %(levelname)s - %(message)s",
    )

    logging.info(f"Starting worker for binary: {args.binary}")
    logging.info("Initializing Connect server (IDA database will open on demand)")

    ida = IDAWrapper(args.binary, args.session_id)
    server = ConnectServer(ida)

    def handle_request(method: str, path: str, data: bytes) -> bytes:
        return server.handle(method, path, data)

    try:
        if args.socket:
            sock = make_unix_socket(args.socket)
            label = args.socket
        else:
            sock = make_tcp_socket(args.port)
            label = f"127.0.0.1:{args.port}"

        serve(sock, handle_request, args.session_id, label)

        # Cleanup Unix socket file on exit
        if args.socket and os.path.exists(args.socket):
            os.remove(args.socket)

    except KeyboardInterrupt:
        logging.info("Shutting down...")
    finally:
        ida.close_database()
        logging.info("Worker terminated")


if __name__ == "__main__":
    main()
