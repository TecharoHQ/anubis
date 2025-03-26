(()=>{function p(s,o=5,t=navigator.hardwareConcurrency||1){return console.debug("fast algo"),new Promise((e,r)=>{let n=URL.createObjectURL(new Blob(["(",w(),")()"],{type:"application/javascript"})),a=[];for(let i=0;i<t;i++){let c=new Worker(n);c.onmessage=l=>{a.forEach(u=>u.terminate()),c.terminate(),e(l.data)},c.onerror=l=>{c.terminate(),r()},c.postMessage({data:s,difficulty:o,nonce:i,threads:t}),a.push(c)}URL.revokeObjectURL(n)})}function w(){return function(){let s=t=>{let e=new TextEncoder().encode(t);return crypto.subtle.digest("SHA-256",e.buffer)};function o(t){return Array.from(t).map(e=>e.toString(16).padStart(2,"0")).join("")}addEventListener("message",async t=>{let e=t.data.data,r=t.data.difficulty,n,a=t.data.nonce,i=t.data.threads;for(;;){let c=await s(e+a),l=new Uint8Array(c),u=!0;for(let d=0;d<r;d++){let f=Math.floor(d/2),m=d%2;if((l[f]>>(m===0?4:0)&15)!==0){u=!1;break}}if(u){n=o(l),console.log(n);break}a+=i}postMessage({hash:n,data:e,difficulty:r,nonce:a})})}.toString()}function g(s,o=5,t=1){return console.debug("slow algo"),new Promise((e,r)=>{let n=URL.createObjectURL(new Blob(["(",y(),")()"],{type:"application/javascript"})),a=new Worker(n);a.onmessage=i=>{a.terminate(),e(i.data)},a.onerror=i=>{a.terminate(),r()},a.postMessage({data:s,difficulty:o}),URL.revokeObjectURL(n)})}function y(){return function(){let s=o=>{let t=new TextEncoder().encode(o);return crypto.subtle.digest("SHA-256",t.buffer).then(e=>Array.from(new Uint8Array(e)).map(r=>r.toString(16).padStart(2,"0")).join(""))};addEventListener("message",async o=>{let t=o.data.data,e=o.data.difficulty,r,n=0;do r=await s(t+n++);while(r.substring(0,e)!==Array(e+1).join("0"));n-=1,postMessage({hash:r,data:t,difficulty:e,nonce:n})})}.toString()}var b={fast:p,slow:g},L=(s="",o={})=>{let t=new URL(s,window.location.href);return Object.entries(o).forEach(e=>{let[r,n]=e;t.searchParams.set(r,n)}),t.toString()},h=(s,o)=>L(`/.within.website/x/cmd/anubis/static/img/${s}.webp`,{cacheBuster:o});(async()=>{let s=document.getElementById("status"),o=document.getElementById("image"),t=document.getElementById("title"),e=document.getElementById("spinner"),r=JSON.parse(document.getElementById("anubis_version").textContent),n=document.getElementById("pass_form");s.innerHTML="Calculating...";let{challenge:a,rules:i}=await fetch("/.within.website/x/cmd/anubis/api/make-challenge",{method:"POST"}).then(m=>{if(!m.ok)throw new Error("Failed to fetch config");return m.json()}).catch(m=>{throw t.innerHTML="Oh no!",s.innerHTML=`Failed to fetch config: ${m.message}`,o.src=h("sad",r),e.innerHTML="",e.style.display="none",m}),c=b[i.algorithm];if(!c){t.innerHTML="Oh no!",s.innerHTML="Failed to resolve check algorithm. You may want to reload the page.",o.src=h("sad",r),e.innerHTML="",e.style.display="none";return}s.innerHTML=`Calculating...<br/>Difficulty: ${i.report_as}`;let l=Date.now(),{hash:u,nonce:d}=await c(a,i.difficulty),f=Date.now();console.log({hash:u,nonce:d}),t.innerHTML="Success!",s.innerHTML=`Done! Took ${f-l}ms, ${d} iterations`,o.src=h("happy",r),e.innerHTML="",e.style.display="none",setTimeout(()=>{n.response.value=u,n.nonce.value=d,n.elapsedTime.value=f-l,n.submit()},250)})();})();
//# sourceMappingURL=main.mjs.map
