#!/usr/bin/env python3
"""Run a local cluster, or verify six real processes and always stop them."""
import argparse
import concurrent.futures
import json
import pathlib
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.request

ROOT = pathlib.Path(__file__).resolve().parents[1]


def http(base, path, payload=None):
    data = None if payload is None else json.dumps(payload).encode()
    request = urllib.request.Request(base + path, data, {"Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(request, timeout=2) as response:
            return response.status, json.load(response)
    except urllib.error.HTTPError as error:
        return error.code, json.load(error)


def wait_for(check, description, timeout=20):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        try:
            if check():
                return
        except (OSError, ValueError):
            pass
        time.sleep(0.1)
    raise RuntimeError("Timed out: " + description)


def verify(urls, stop=None, start=None):
    for url in urls:
        wait_for(lambda: http(url, "/healthz")[0] == 200, "healthy " + url)
    initial_chains = [http(url, "/chain")[1] for url in urls]
    transaction = {"id": f"smoke-{time.time_ns()}", "data": 'Quoted "message" stays intact',
                   "timestamp": int(time.time())}
    offline = len(urls) - 1
    if stop:
        stop(offline)
    code, response = http(urls[0], "/transaction", transaction)
    assert code == 202, (code, response)

    def converged(members, txid):
        chains = [http(url, "/chain")[1] for url in members]
        if len({chain[-1]["hash"] for chain in chains}) != 1:
            return False
        if not all(any(tx["id"] == txid for block in chain for tx in block["transactions"])
                   for chain in chains):
            return False
        return all(http(url, "/mempool")[1]["count"] == 0 for url in members)

    active = urls[:-1] if stop else urls
    wait_for(lambda: converged(active, transaction["id"]), "transaction reaches active peers")
    print("PASS: active nodes converge and clear confirmed messages", flush=True)
    if start:
        start(offline)
        wait_for(lambda: converged(urls, transaction["id"]), "offline node catches up without a new transaction")
        print("PASS: restarted node catches up while the network is idle", flush=True)
    assert http(urls[1], "/transaction", transaction)[0] == 409, "confirmed replay accepted"
    assert http(urls[1], "/block", {"hash": "x", "prev_hash": "x"})[0] == 400, "forged block accepted"
    lookup = http(urls[2], "/transaction/" + transaction["id"])[1]
    assert lookup["status"] == "confirmed" and lookup["transaction"] == transaction
    print("PASS: replay/forgery rejected and complete message preserved", flush=True)

    duplicate = {"id": f"duplicate-{time.time_ns()}", "data": "one submission", "timestamp": int(time.time())}
    with concurrent.futures.ThreadPoolExecutor(max_workers=8) as pool:
        codes = list(pool.map(lambda _: http(urls[0], "/transaction", duplicate)[0], range(8)))
    assert codes.count(202) == 1 and codes.count(409) == 7, codes
    wait_for(lambda: converged(urls, duplicate["id"]), "concurrent submission converges")
    print("PASS: eight simultaneous requests accept exactly one transaction", flush=True)
    return initial_chains


def free_ports(count):
    sockets = []
    try:
        for _ in range(count):
            listener = socket.socket()
            listener.bind(("127.0.0.1", 0))
            sockets.append(listener)
        return [listener.getsockname()[1] for listener in sockets]
    finally:
        for listener in sockets:
            listener.close()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--nodes", type=int, choices=range(1, 7), default=3)
    parser.add_argument("--smoke", action="store_true")
    parser.add_argument("--existing", action="store_true", help="verify Docker nodes on ports 8081–8086")
    args = parser.parse_args()
    if args.existing:
        verify([f"http://127.0.0.1:{8081+i}" for i in range(6)])
        return
    count = 6 if args.smoke else args.nodes
    binary = ROOT / "bin/node"
    if not binary.exists():
        raise SystemExit("Build the binary first: make build")
    ports = free_ports(count) if args.smoke else list(range(8081, 8081 + count))
    urls = [f"http://127.0.0.1:{port}" for port in ports]
    with tempfile.TemporaryDirectory(prefix="p2pledger-cluster-") as temporary:
        state = pathlib.Path(temporary) if args.smoke else ROOT / "data/local"
        state.mkdir(parents=True, exist_ok=True)
        processes, logs = {}, {}

        def start(index):
            peer_file = pathlib.Path(temporary) / f"peers-{index}.txt"
            peer_file.write_text("\n".join(url for i, url in enumerate(urls) if i != index) + "\n")
            logs[index] = open(state / f"node{index+1}.log", "ab", buffering=0)
            command = [str(binary), "-port", str(ports[index]), "-addr", urls[index],
                       "-peers", str(peer_file), "-chain", str(state / f"chain{index+1}.json")]
            if args.smoke:
                command += ["-difficulty", "1", "-mine-interval", "100ms", "-sync-interval", "200ms"]
            processes[index] = subprocess.Popen(command, cwd=ROOT, stdout=logs[index], stderr=subprocess.STDOUT)

        def stop(index):
            process = processes.pop(index, None)
            if process is not None:
                process.terminate()
                try:
                    process.wait(timeout=7)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()
            log = logs.pop(index, None)
            if log:
                log.close()

        try:
            for index in range(count):
                start(index)
            if args.smoke:
                verify(urls, stop, start)
            else:
                for url in urls:
                    wait_for(lambda: http(url, "/healthz")[0] == 200, "healthy " + url)
                print("Nodes: " + ", ".join(urls) + "\nCtrl+C stops the entire cluster.", flush=True)
                while True:
                    if any(process.poll() is not None for process in processes.values()):
                        raise RuntimeError("A node exited; inspect data/local/node*.log")
                    time.sleep(0.5)
        except KeyboardInterrupt:
            pass
        except Exception:
            for index in range(count):
                log_file = state / f"node{index+1}.log"
                if log_file.exists():
                    print(f"--- Node {index+1} ---\n{log_file.read_text()[-6000:]}")
            raise
        finally:
            for index in list(processes):
                stop(index)
            print("All cluster processes stopped.", flush=True)


if __name__ == "__main__":
    main()
