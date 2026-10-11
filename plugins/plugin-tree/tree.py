#!/usr/bin/env python3
"""Plugin dependency tree and how each plugin talks to jig. Usage: tree.py tree|edges"""
import json, os, re, shutil, subprocess, sys


def ps():
    out = subprocess.run(["ps", "-axo", "pid=,ppid=,rss=,pcpu=,command="], capture_output=True, text=True).stdout
    procs = {}
    for line in out.splitlines():
        f = line.split(None, 4)
        if len(f) == 5:
            procs[int(f[0])] = {"pid": int(f[0]), "ppid": int(f[1]), "mb": int(f[2]) / 1024, "cpu": float(f[3]), "cmd": f[4]}
    return procs


def jig_pid(procs):
    pid = os.getppid()
    while pid in procs:
        exe = os.path.basename(procs[pid]["cmd"].split()[0])
        if exe == "jig":
            return pid
        pid = procs[pid]["ppid"]
    return os.getppid()


def plugins(jig_exe):
    cmd = [jig_exe]
    if os.environ.get("JIG_FILE"):
        cmd += ["-f", os.environ["JIG_FILE"]]
    if os.environ.get("JIG_ENV"):
        cmd += ["-e", os.environ["JIG_ENV"]]
    out = subprocess.run(cmd + ["plugin", "ls"], capture_output=True, text=True, env={**os.environ, "NO_COLOR": "1"}).stdout
    rows = []
    for line in out.splitlines()[1:]:
        f = line.split(None, 4)
        if len(f) >= 5:
            rows.append({"name": f[0], "version": f[1], "kind": f[2], "state": f[3], "dir": re.sub(r"\s+\(.*\)$", "", f[4])})
    return rows


def manifest(p):
    try:
        text = open(os.path.join(p["dir"], "jig-plugin.yaml")).read()
    except OSError:
        return [], "?"
    req = []
    m = re.search(r"^requires:\s*(\{.*\})?\s*\n((?:[ \t]+\S.*\n?)*)", text, re.M)
    if m:
        body = m.group(1) or m.group(2)
        req = re.findall(r"([a-z][a-z0-9-]*)\s*:", body)
    if re.search(r"^server:", text, re.M):
        talks = "JSON-RPC over stdio"
    elif re.search(r"^mcp:", text, re.M):
        talks = "MCP over stdio"
    elif re.search(r"^\s+run:", text, re.M):
        talks = "scripts (run:)"
    elif req and not re.search(r"^(views|sources|actions):", text, re.M):
        talks = "pack (only requires)"
    else:
        talks = "yaml only"
    return req, talks


def main():
    mode = sys.argv[1] if len(sys.argv) > 1 else "tree"
    procs = ps()
    jp = jig_pid(procs)
    jig_exe = shutil.which("jig") or procs.get(jp, {}).get("cmd", "jig").split()[0]
    ps_list = plugins(jig_exe)
    by = {p["name"]: p for p in ps_list}
    children = {}
    for pid, pr in procs.items():
        children.setdefault(pr["ppid"], []).append(pid)

    def tree_of(pid):
        out, stack = [], [pid]
        while stack:
            x = stack.pop()
            out.append(x)
            stack += children.get(x, [])
        return out

    for p in ps_list:
        p["requires"], p["talks"] = manifest(p)
        p["procs"] = []
    for c in children.get(jp, []):
        cmd = procs[c]["cmd"]
        for p in ps_list:
            if re.search(r"jig-std\s+%s(\s|$)" % re.escape(p["name"]), cmd) or (p["dir"] and p["dir"] in cmd):
                p["procs"] += tree_of(c)
    for p in ps_list:
        p["pids"] = sorted(set(p["procs"]))
        p["mb"] = sum(procs[q]["mb"] for q in p["pids"])
        p["cpu"] = sum(procs[q]["cpu"] for q in p["pids"])
        p["use"] = "off" if p["state"] != "on" else ("running" if p["pids"] else "idle")

    if mode == "edges":
        jig = procs.get(jp)
        for p in ps_list:
            if p["state"] == "on":
                print(json.dumps({"from": "jig", "to": p["name"], "label": p["talks"]}))
            for r in p["requires"]:
                print(json.dumps({"from": p["name"], "to": r, "label": "requires"}))
        return

    required = {r for p in ps_list for r in p["requires"]}
    rows = []
    jig = procs.get(jp, {"mb": 0, "cpu": 0, "pid": jp})
    own = sum(procs[q]["mb"] for q in tree_of(jp))
    rows.append({"id": "jig", "parent": "", "name": "jig", "use": "running", "talks": "the host", "kind": "host",
                 "pid": jp, "cpu": round(jig["cpu"], 1), "mem": "%.0f MB (+children %.0f)" % (jig["mb"], own - jig["mb"]),
                 "version": "", "dir": ""})

    def add(name, parent, seen):
        p = by.get(name)
        rid = parent + "/" + name
        if not p:
            rows.append({"id": rid, "parent": parent, "name": name, "use": "missing", "talks": "?", "kind": "", "pid": "",
                         "cpu": "", "mem": "", "version": "", "dir": ""})
            return
        rows.append({"id": rid, "parent": parent, "name": name, "use": p["use"], "talks": p["talks"], "kind": p["kind"],
                     "pid": ",".join(map(str, p["pids"][:3])), "cpu": round(p["cpu"], 1) if p["pids"] else "",
                     "mem": "%.0f MB" % p["mb"] if p["pids"] else "", "version": p["version"], "dir": p["dir"]})
        for r in p["requires"]:
            if r not in seen:
                add(r, rid, seen | {name})

    for p in ps_list:
        if p["name"] not in required:
            add(p["name"], "jig", {p["name"]})
    for r in rows:
        print(json.dumps(r))


main()
