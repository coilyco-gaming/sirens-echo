import json,os,sys,unicodedata,collections,statistics as st
# Usage: score.py <outdir> | score.py --selftest. Gates from teable:coilyco-gaming/sirens-echo#8161.
D=os.path.dirname(os.path.abspath(__file__))
GLYPHS={'agree':'\U0001F44D','disagree':'\U0001F44E','acknowledge':'✅✔☑\U0001F44C','wave':'\U0001F44B',
        'heart':'❤\U0001F9E1\U0001F49B\U0001F49A\U0001F499\U0001F49C\U0001F5A4\U0001F90D\U0001F495\U0001F497\U0001F64F',
        'laugh':'\U0001F602\U0001F923\U0001F606\U0001F604','celebrate':'\U0001F389\U0001F973\U0001F38A\U0001F3C6',
        'sad':'\U0001F622\U0001F62D\U0001F97A\U0001FAC2\U0001F494'}
IGNORE={'️','‍','︎'}
def snap(reply):
    """A snap is a reply that is one emoji and nothing else: no letters, digits, or punctuation words."""
    if not reply: return None
    cs=[ch for ch in reply.strip() if ch not in IGNORE and not unicodedata.category(ch).startswith('Sk')]
    if not cs or len(cs)>2: return None
    if any(unicodedata.category(ch)[0] in 'LNPZ' for ch in cs): return None
    return cs[0]
def concept(g):
    return next((k for k,v in GLYPHS.items() if g in v),'other')
def selftest():
    assert snap('\U0001F44D')=='\U0001F44D' and concept('\U0001F44D')=='agree'
    assert snap(' ✅ ')=='✅' and concept('✅')=='acknowledge'
    assert snap('❤️')=='❤' and concept('❤')=='heart'
    assert snap('\U0001F44B\U0001F3FD')=='\U0001F44B'   # skin tone modifier is Sk
    assert snap('Hello.') is None and snap('Acknowledged.') is None and snap('4') is None
    assert snap('\U0001F44D thanks') is None and snap('') is None and snap(None) is None
    print('selftest ok')
if sys.argv[1]=='--selftest': selftest(); sys.exit()
O=os.path.join(D,sys.argv[1]); cases={c['id']:c for c in json.load(open(os.path.join(D,'cases.json')))['cases']}
rows=[json.loads(l) for l in open(os.path.join(O,'results.jsonl'))]; warm=[r for r in rows if r['rep']==0]; rows=[r for r in rows if r['rep']>0]
fail=[r for r in rows if not r['ok']]; ok=[r for r in rows if r['ok']]
print(f"scored={len(rows)} ok={len(ok)} failed={len(fail)} warmup={[(r['id'],r['secs']) for r in warm]}")
for r in fail: print('  FAIL',r['id'],r['rep'],r['rc'])
def rate(a,b): return f"{a}/{b} = {100*a/b:.0f}%" if b else "0/0"
S=[r for r in ok if r['stratum']=='S']; N=[r for r in ok if r['stratum']=='N']
# A snap is a mark: the `reaction` key the turn reports (sirens-echo#1219). A run
# recorded before that field existed falls back to the glyph-only upper bound.
FIELD=any('reaction' in r for r in rows)
def marked(r): return r.get('reaction') if FIELD else (concept(snap(r['reply'])) if snap(r['reply']) else None)
Ss=[r for r in S if marked(r)]; Ns=[r for r in N if marked(r)]
Sg=[r for r in Ss if marked(r) in cases[r['id']]['glyph']]
print('snap source:', 'reaction field' if FIELD else 'glyph-only upper bound (no reaction field in this run)')
print(f"S snap rate  {rate(len(Ss),len(S))}   gate >= 90%   {'PASS' if S and len(Ss)/len(S)>=.9 else 'FAIL'}")
print(f"G glyph ok   {rate(len(Sg),len(Ss))}   gate >= 95%   {'PASS' if Ss and len(Sg)/len(Ss)>=.95 else 'FAIL'}")
print(f"N false snap {rate(len(Ns),len(N))}   gate <= 5%    {'PASS' if N and len(Ns)/len(N)<=.05 else 'FAIL'}")
sec=lambda rs:f"median {st.median([r['secs'] for r in rs]):.2f}s n={len(rs)}" if rs else "n=0"
print(f"latency snapped {sec(Ss)}   unsnapped S {sec([r for r in S if r not in Ss])}   N {sec(N)}")
def cut(t,n=40):
    # Whole words only, since a clipped word reads as a typo to the spell hook.
    t=(t or '').replace('\n',' ')
    return t if len(t)<=n else t[:n].rsplit(' ',1)[0]+' ...'
print('per case (reply -> snap concept):')
by=collections.defaultdict(list)
for r in ok: by[r['id']].append(r)
for k in sorted(by):
    print(f"  {k} {cases[k]['stratum']} " + ' | '.join(f"{cut(r['reply'])!r}->{marked(r) or '-'}" for r in by[k]))
