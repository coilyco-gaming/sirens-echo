import json,random,subprocess,sys,time,os
D=os.path.dirname(os.path.abspath(__file__))
P=['hi, quick check: what is 2+2?','Say hello back in one short sentence.','In about 80 words, explain what a tidal lock is.']
plan=[(l,p,r) for l in ('echo','deep') for p in range(3) for r in range(1,21)]
random.Random(8139).shuffle(plan)
plan=[('echo',0,0),('deep',0,0)]+plan  # rep 0 = warm-up, discarded
out=open(os.path.join(D,'results.jsonl'),'a')
for i,(lane,p,r) in enumerate(plan):
    args=json.dumps({'author':'perf-test','content':P[p]})
    t0=time.time()
    cp=subprocess.run(['mcporter','call',f'tailnet_coilyco_sirens_{lane}.turn','--timeout','300000','--output','json','--args',args],capture_output=True,text=True)
    t1=time.time()
    reply=None
    try: reply=json.loads(cp.stdout).get('reply')
    except Exception: pass
    raw=os.path.join(D,'raw',f'{i:03d}-{lane}-p{p}-r{r}.txt'); open(raw,'w').write(cp.stdout+cp.stderr)
    rec=dict(i=i,lane=lane,p=p,rep=r,start=t0,end=t1,secs=round(t1-t0,3),rc=cp.returncode,
             ok=cp.returncode==0 and bool(reply),reply_bytes=len(reply.encode()) if reply else 0,
             tool_footer=bool(reply and '🔨' in reply))
    out.write(json.dumps(rec)+'\n'); out.flush()
    print(i,lane,f'p{p}',r,rec['secs'],rec['rc'],rec['reply_bytes'],flush=True)
