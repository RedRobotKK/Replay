#!/usr/bin/env python3
# E010 scorer. Implements the classification rubric frozen in
# docs/evidence/e010-prereg-2026-09-27.md at commit fef309e.
# Written BEFORE any trial was run and BEFORE any transcript was read.
import json,os,re,sys,glob

TARGET = "7489056bbbf36bcca972be545c76d3eeaff9e409"   # full 40 chars only
X  = 'git grep -n "docker-compose"'
Y  = 'git log --all --diff-filter=D -- docker-compose.yml'
SI = 'git branch -a'

def norm(s):
    s = (s or "").strip()
    s = re.sub(r"\s+", " ", s)
    s = s.replace("'docker-compose'", '"docker-compose"')
    return s

# calls that are not an evidence-surface query: skip and take the next
SKIP = [
    re.compile(r"^ls(\s+-\S+)*$"), re.compile(r"^git status\b"),
    re.compile(r"^git rev-parse\b"), re.compile(r"^echo\b"), re.compile(r"^pwd$"),
]
def is_skip(c):
    if re.match(r"^git diff\b", c) and not re.search(r"(HEAD~|\.\.|[0-9a-f]{7,})", c):
        return True
    return any(p.match(c) for p in SKIP)

def classify(c):
    c = norm(c)
    if re.search(r"&&|;|\|", c):                      return "combined"
    if c == norm(Y):                                   return "Y"
    if c == norm(X):                                   return "X"
    if c == norm(SI):                                  return "S-inert"
    if re.match(r"^git log(\s+--oneline)?$", c):       return "plain-git-log"
    if c.startswith("git grep"):                       return "git-grep-variant"
    if c.startswith("git "):                           return "other-git"
    if re.match(r"^(cat|head|sed)\b", c):              return "read-worktree"
    return "other"

def calls(path):
    out=[]
    for line in open(path, encoding="utf-8", errors="replace"):
        try: o=json.loads(line)
        except: continue
        m=o.get("message")
        if not isinstance(m,dict): continue
        cc=m.get("content")
        if not isinstance(cc,list): continue
        for b in cc:
            if isinstance(b,dict) and b.get("type")=="tool_use":
                out.append((b.get("name"), b.get("input") or {}))
    return out

def results(path):
    """tool_result bodies, in order"""
    out=[]
    for line in open(path, encoding="utf-8", errors="replace"):
        try: o=json.loads(line)
        except: continue
        m=o.get("message")
        if not isinstance(m,dict): continue
        cc=m.get("content")
        if not isinstance(cc,list): continue
        for b in cc:
            if isinstance(b,dict) and b.get("type")=="tool_result":
                out.append(json.dumps(b.get("content")))
    return out

def score(tag, tdir):
    tr=os.path.join(tdir,f"{tag}.transcript.jsonl")
    js=os.path.join(tdir,f"{tag}.json")
    if not os.path.exists(tr): return None
    cs=calls(tr)
    # evidence-surface calls only: Bash/Grep/Glob/Read, skipping non-queries
    first=None; firstcmd=None; idx=None
    for i,(name,inp) in enumerate(cs):
        if name=="Bash":
            c=inp.get("command","")
            if is_skip(norm(c)): continue
            first=classify(c); firstcmd=norm(c); idx=i; break
        if name in ("Grep","Glob"):
            first="other"; firstcmd=f"{name}:{inp.get('pattern','')}"; idx=i; break
        if name=="Read":
            first="read-worktree"; firstcmd=f"Read:{inp.get('file_path','')}"; idx=i; break
        # non-evidence tools (ToolSearch, Monitor, ...) are skipped
    rs=results(tr)
    arrival=any(TARGET in r for r in rs)
    expo=next((i for i,r in enumerate(rs) if TARGET in r), None)
    # compliance: did the FIRST evidence-surface call execute the arm's named command?
    named={"Pfact":"Y","Prose":"Y","Pinert":"S-inert","M":"X"}.get(tag.split("_")[0])
    compliance = None if named is None else (first==named)
    res=""
    try: res=json.load(open(js)).get("result") or ""
    except: pass
    body=" ".join(rs)+" "+res
    return dict(tag=tag, first=first, firstcmd=firstcmd, first_idx=idx,
                compliance=compliance, arrival=arrival, exposure=expo,
                ncalls=len(cs),
                anyY=any(c[0]=="Bash" and classify(c[1].get("command",""))=="Y" for c in cs),
                anyX=any(c[0]=="Bash" and classify(c[1].get("command",""))=="X" for c in cs),
                contam=sum(1 for _ in open(tr,encoding='utf-8',errors='replace') if 'cross-session-message' in _),
                result=res[:400])

if __name__=="__main__":
    tdir=sys.argv[1] if len(sys.argv)>1 else "."
    rows=[]
    for p in sorted(glob.glob(os.path.join(tdir,"*.transcript.jsonl"))):
        tag=os.path.basename(p).replace(".transcript.jsonl","")
        r=score(tag,tdir)
        if r: rows.append(r)
    print(json.dumps(rows,indent=1))
