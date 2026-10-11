#!/usr/bin/env python3
"""CPU and memory of jig and every process under it, grouped by plugin."""
import json, os, re, subprocess

out = subprocess.run(["ps", "-axo", "pid=,ppid=,rss=,pcpu=,etime=,command="], capture_output=True, text=True).stdout
procs = {}
for line in out.splitlines():
    f = line.split(None, 5)
    if len(f) == 6:
        procs[int(f[0])] = {"pid": int(f[0]), "ppid": int(f[1]), "mb": int(f[2]) / 1024, "cpu": float(f[3]), "up": f[4], "cmd": f[5]}

pid = os.getppid()
while pid in procs and os.path.basename(procs[pid]["cmd"].split()[0]) != "jig":
    pid = procs[pid]["ppid"]
jig = pid if pid in procs else os.getppid()

kids = {}
for p in procs.values():
    kids.setdefault(p["ppid"], []).append(p["pid"])


def owner(p, top):
    m = re.search(r"jig-std\s+([a-z0-9-]+)", p["cmd"]) or re.search(r"/plugins/([a-z0-9-]+)/", p["cmd"])
    return m.group(1) if m else ("jig" if p["pid"] == jig else ("task" if top == p["pid"] else None))


groups = {}
for top in kids.get(jig, []):
    stack = [top]
    name = owner(procs[top], top) or "other"
    while stack:
        x = stack.pop()
        g = groups.setdefault(name, {"name": name, "procs": 0, "cpu": 0.0, "mb": 0.0, "up": procs[x]["up"]})
        g["procs"] += 1
        g["cpu"] += procs[x]["cpu"]
        g["mb"] += procs[x]["mb"]
        stack += kids.get(x, [])
me = procs.get(jig)
rows = [{"name": "jig", "procs": 1, "cpu": me["cpu"], "mb": me["mb"], "up": me["up"]}] if me else []
rows += sorted(groups.values(), key=lambda g: -g["mb"])
total = {"name": "total", "procs": sum(r["procs"] for r in rows), "cpu": sum(r["cpu"] for r in rows), "mb": sum(r["mb"] for r in rows), "up": ""}
for r in rows + [total]:
    print(json.dumps({"name": r["name"], "procs": r["procs"], "cpu %": round(r["cpu"], 1), "memory MB": round(r["mb"], 1), "up": r["up"]}))
