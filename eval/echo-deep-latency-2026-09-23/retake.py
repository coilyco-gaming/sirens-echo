import json,subprocess,time,os
D=os.path.dirname(os.path.abspath(__file__))
P=['hi, quick check: what is 2+2?','Say hello back in one short sentence.','In about 80 words, explain what a tidal lock is.']
cells=[('echo',1,4),('echo',2,18),('echo',2,1)]
out=open(os.path.join(D,'results.jsonl'),'a'); i0=sum(1 for _ in open(os.path.join(D,'results.jsonl')))
for k,(lane,p,r) in enumerate(cells):
    i=i0+k; t0=time.time()
    cp=subprocess.run(['mcporter','call',f'tailnet_coilyco_sirens_{lane}.turn','--timeout','300000','--output','json','--args',json.dumps({'author':'perf-test','content':P[p]})],capture_output=True,text=True)
    t1=time.time(); reply=None
    try: reply=json.loads(cp.stdout).get('reply')
    except Exception: pass
    open(os.path.join(D,'raw',f'{i:03d}-{lane}-p{p}-r{r}.txt'),'w').write(cp.stdout+cp.stderr)
    rec=dict(i=i,lane=lane,p=p,rep=r,start=t0,end=t1,secs=round(t1-t0,3),rc=cp.returncode,ok=cp.returncode==0 and bool(reply),reply_bytes=len(reply.encode()) if reply else 0,tool_footer=bool(reply and '🔨' in reply),retake=True)
    out.write(json.dumps(rec)+'\n'); out.flush(); print(rec,flush=True)
