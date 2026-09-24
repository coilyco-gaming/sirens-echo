import json,random,statistics as st,os,sys,collections
D=os.path.dirname(os.path.abspath(__file__))
rows=[json.loads(l) for l in open(os.path.join(D,'results.jsonl'))]
import datetime as dt
LO=dt.datetime(2026,9,23,23,48,50,tzinfo=dt.timezone.utc).timestamp(); HI=dt.datetime(2026,9,23,23,50,14,tzinfo=dt.timezone.utc).timestamp()
RETAKES='--retakes' in sys.argv
warm=[r for r in rows if r['rep']==0]; rows=[r for r in rows if r['rep']>0]
dropped=[]
print('dropped (ollama probe window):',[(r['i'],r['p'],r['rep'],r['secs']) for r in dropped])
rows=[r for r in rows if r not in dropped and (RETAKES or not r.get('retake'))]
print('mode:', 'with retakes' if RETAKES else 'without retakes')
fail=[r for r in rows if not r['ok']]; ok=[r for r in rows if r['ok']]
print(f"scored={len(rows)} ok={len(ok)} failed={len(fail)} warmup={[(r['lane'],r['secs']) for r in warm]}")
for r in fail: print('  FAIL',r['i'],r['lane'],r['p'],r['rep'],r['rc'],r['secs'])
def q(v,x):
    v=sorted(v); k=(len(v)-1)*x; f=int(k); c=min(f+1,len(v)-1); return v[f]+(v[c]-v[f])*(k-f)
def boot(v,n=5000,seed=8139):
    R=random.Random(seed); m=sorted(st.median([R.choice(v) for _ in v]) for _ in range(n)); return m[int(.025*n)],m[int(.975*n)]
def rank(v): s=sorted(range(len(v)),key=lambda i:v[i]); r=[0]*len(v); [r.__setitem__(j,i) for i,j in enumerate(s)]; return r
def spear(a,b):
    if len(a)<3 or len(set(a))<2 or len(set(b))<2: return float('nan')
    ra,rb=rank(a),rank(b); return st.correlation(ra,rb)
for lane in ('echo','deep'):
  for p in range(3):
    c=[r for r in ok if r['lane']==lane and r['p']==p]
    if not c: continue
    v=[r['secs'] for r in c]; b=[r['reply_bytes'] for r in c]
    m=st.mean(v); sd=st.stdev(v) if len(v)>1 else 0
    lo,hi=boot(v) if len(v)>1 else (v[0],v[0])
    distinct=len(set(open(os.path.join(D,'raw',f"{r['i']:03d}-{lane}-p{p}-r{r['rep']}.txt")).read() for r in c))
    tf=sum(r['tool_footer'] for r in c)
    calls=[open(os.path.join(D,'raw',f"{r['i']:03d}-{lane}-p{p}-r{r['rep']}.txt")).read().count('> \\ud83d') + open(os.path.join(D,'raw',f"{r['i']:03d}-{lane}-p{p}-r{r['rep']}.txt")).read().count('> 🔨')+open(os.path.join(D,'raw',f"{r['i']:03d}-{lane}-p{p}-r{r['rep']}.txt")).read().count('> 📖') for r in c]
    print(f"{lane} p{p} n={len(v)} median={st.median(v):.2f} [95%CI {lo:.2f}-{hi:.2f}] p90={q(v,.9):.2f} min={min(v):.2f} max={max(v):.2f} sd={sd:.2f} cv={sd/m*100:.1f}% distinct_replies={distinct} tool_footer={tf}/{len(v)} rho(secs,bytes)={spear(v,b):.2f} rho(secs,order)={spear(v,[r['i'] for r in c]):.2f} rho(secs,toolcalls)={spear(v,calls):.2f}")
    print('   secs:toolcalls',' '.join(f'{x:.1f}:{k}' for x,k in sorted(zip(v,calls))))
    print('   sorted secs:',' '.join(f'{x:.1f}' for x in sorted(v)))
