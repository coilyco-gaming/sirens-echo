import json,random,subprocess,sys,time,os
# Usage: run.py <outdir> [reps]. One echo turn per case per rep, shuffled with seed 8161.
D=os.path.dirname(os.path.abspath(__file__)); OUT=os.path.join(D,sys.argv[1]); REPS=int(sys.argv[2]) if len(sys.argv)>2 else 2
cases=json.load(open(os.path.join(D,'cases.json')))['cases']
plan=[(c,r) for c in cases for r in range(1,REPS+1)]; random.Random(8161).shuffle(plan)
plan=[(cases[-1],0)]+plan  # rep 0 = warm-up, discarded
os.makedirs(os.path.join(OUT,'raw'),exist_ok=True); out=open(os.path.join(OUT,'results.jsonl'),'a')
for i,(c,r) in enumerate(plan):
    args=json.dumps({'author':'perf-test','content':c['content']})
    t0=time.time()
    cp=subprocess.run(['mcporter','call','tailnet_coilyco_sirens_echo.turn','--timeout','300000','--output','json','--args',args],capture_output=True,text=True)
    t1=time.time(); reply=None; reaction=None
    try: out_=json.loads(cp.stdout); reply=out_.get('reply'); reaction=out_.get('reaction')
    except Exception: pass
    open(os.path.join(OUT,'raw',f"{i:03d}-{c['id']}-r{r}.txt"),'w').write(cp.stdout+cp.stderr)
    rec=dict(i=i,id=c['id'],stratum=c['stratum'],rep=r,start=t0,secs=round(t1-t0,3),rc=cp.returncode,ok=cp.returncode==0 and reply is not None,reply=reply,reaction=reaction)
    out.write(json.dumps(rec,ensure_ascii=False)+'\n'); out.flush()
    print(i,c['id'],r,rec['secs'],rec['rc'],repr(reply)[:60],flush=True)
