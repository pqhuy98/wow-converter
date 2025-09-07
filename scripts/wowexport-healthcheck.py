#!/usr/bin/env python3
import socket
import sys
import json


def send_rcp(host: str, port: int, payload: dict, timeout: float = 5.0):
    data = json.dumps(payload).encode("utf-8")
    header = f"{len(data)}\0".encode("utf-8")
    s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    s.settimeout(timeout)
    try:
        s.connect((host, port))
        s.sendall(header + data)
        # Read header (length before null byte)
        buf = b""
        while b"\0" not in buf:
            chunk = s.recv(1)
            if not chunk:
                raise RuntimeError("Connection closed before header delimiter")
            buf += chunk
        size_part, remainder = buf.split(b"\0", 1)
        try:
            size = int(size_part.decode("utf-8"))
        except Exception:
            raise RuntimeError("Invalid header length returned from server")
        body = remainder
        while len(body) < size:
            chunk = s.recv(size - len(body))
            if not chunk:
                raise RuntimeError("Connection closed before full body received")
            body += chunk
        return json.loads(body.decode("utf-8"))
    finally:
        s.close()


def main():
    host = sys.argv[1] if len(sys.argv) > 1 else "127.0.0.1"
    port = int(sys.argv[2]) if len(sys.argv) > 2 else 17751

    # Ask for CONFIG_GET to verify server responds
    resp = send_rcp(host, port, {"id": "CONFIG_GET"})
    print("CONFIG_GET:", json.dumps(resp, indent=2))

    # Ask for CASC_INFO (will return CASC_UNAVAILABLE if not set up yet)
    resp2 = send_rcp(host, port, {"id": "GET_CASC_INFO"})
    print("GET_CASC_INFO:", json.dumps(resp2, indent=2))

    print("OK")


if __name__ == "__main__":
    main()

