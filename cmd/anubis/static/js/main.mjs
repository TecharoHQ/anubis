(()=>{function m(n,i=5,e=navigator.hardwareConcurrency){return new Promise((t,d)=>{let s=URL.createObjectURL(new Blob(["(",p(),")()"],{type:"application/javascript"})),a=[];for(let u=0;u<e;u++){let o=new Worker(s);o.onmessage=r=>{a.forEach(c=>c.terminate()),o.terminate(),t(r.data)},o.onerror=r=>{o.terminate(),d()},o.postMessage({data:n,difficulty:i,nonce:1e6*u}),a.push(o)}URL.revokeObjectURL(s)})}function p(){return function(){let n=e=>{let t=new TextEncoder().encode(e);return crypto.subtle.digest("SHA-256",t.buffer)};function i(e){return Array.from(e).map(t=>t.toString(16).padStart(2,"0")).join("")}addEventListener("message",async e=>{let t=e.data.data,d=e.data.difficulty,s,a=e.data.nonce||0;for(;;){let u=await n(t+a++),o=new Uint8Array(u),r=!0;for(let c=0;c<d;c++){let l=Math.floor(c/2),f=c%2;if((o[l]>>(f===0?4:0)&15)!==0){r=!1;break}}if(r){s=i(o),console.log(s);break}}a-=1,postMessage({hash:s,data:t,difficulty:d,nonce:a})})}.toString()}var w=(n="",i={})=>{let e=new URL(n,window.location.href);return Object.entries(i).forEach(t=>{let[d,s]=t;e.searchParams.set(d,s)}),e.toString()},h=(n,i)=>w(`/.within.website/x/cmd/anubis/static/img/${n}.webp`,{cacheBuster:i});(async()=>{let n=document.getElementById("status"),i=document.getElementById("image"),e=document.getElementById("title"),t=document.getElementById("spinner"),d=JSON.parse(document.getElementById("anubis_version").textContent);n.innerHTML="Calculating...";let{challenge:s,difficulty:a}=await fetch("/.within.website/x/cmd/anubis/api/make-challenge",{method:"POST"}).then(l=>{if(!l.ok)throw new Error("Failed to fetch config");return l.json()}).catch(l=>{throw e.innerHTML="Oh no!",n.innerHTML=`Failed to fetch config: ${l.message}`,i.src=h("sad"),t.innerHTML="",t.style.display="none",l});n.innerHTML=`Calculating...<br/>Difficulty: ${a}`;let u=Date.now(),{hash:o,nonce:r}=await m(s,a),c=Date.now();console.log({hash:o,nonce:r}),e.innerHTML="Success!",n.innerHTML=`Done! Took ${c-u}ms, ${r} iterations`,i.src=h("happy",d),t.innerHTML="",t.style.display="none",setTimeout(()=>{let l=window.location.href;window.location.href=w("/.within.website/x/cmd/anubis/api/pass-challenge",{response:o,nonce:r,redir:l,elapsedTime:c-u})},250)})();})();
//# sourceMappingURL=main.mjs.map
