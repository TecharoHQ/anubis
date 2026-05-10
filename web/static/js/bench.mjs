/*
@licstart  The following is the entire license notice for the
JavaScript code in this page.

Copyright (c) 2025 Xe Iaso <xe.iaso@techaro.lol>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.

Includes code from https://github.com/aws/aws-sdk-js-crypto-helpers which is
used under the terms of the Apache 2 license.

@licend  The above is the entire license notice
for the JavaScript code in this page.
*/
(()=>{var R=()=>navigator.hardwareConcurrency!==void 0?navigator.hardwareConcurrency:1;function y(e,t,n=5,o=null,a,l=Math.trunc(Math.max(R()/2,1))){console.debug("fast algo");let r="purejs";return window.isSecureContext&&(r="webcrypto"),(navigator.userAgent.includes("Firefox")||navigator.userAgent.includes("Goanna"))&&(console.log("Firefox detected, using pure-JS fallback"),r="purejs"),new Promise((c,m)=>{let i=`${e.basePrefix}/.within.website/x/cmd/anubis/static/js/worker/sha256-${r}.mjs?cacheBuster=${e.version}`,u=[],h=!1,L=()=>{console.log("PoW aborted"),x(),m(new DOMException("Aborted","AbortError"))},x=()=>{h||(h=!0,u.forEach(d=>d.terminate()),o?.removeEventListener("abort",L))};if(o!=null){if(o.aborted)return L();o.addEventListener("abort",L,{once:!0})}for(let d=0;d<l;d++){let E=new Worker(i);E.onmessage=p=>{typeof p.data=="number"?a?.(p.data):(x(),c(p.data))},E.onerror=p=>{x(),m(p)},E.postMessage({data:t,difficulty:n,nonce:d,threads:l}),u.push(E)}})}var M={fast:y,slow:y};var C=4,g=document.getElementById("status"),k=document.getElementById("difficulty-input"),S=document.getElementById("algorithm-select"),v=document.getElementById("compare-select"),A=document.getElementById("table-header"),P=document.getElementById("table-header-compare"),s=document.getElementById("results"),F=()=>{if(C!=null){k.value=C.toString();for(let e of Object.keys(M)){let t=document.createElement("option");S?.append(t);let n=document.createElement("option");v.append(n),t.value=t.innerText=n.value=n.innerText=e}}},I=async(e,t,n,o)=>{if(!(t>=1))throw new Error(`Invalid difficulty: ${t}`);let a=M[n];if(a==null)throw new Error(`Unknown algorithm: ${n}`);let l=new Uint8Array(32);crypto.getRandomValues(l);let r=Array.from(l).map(h=>h.toString(16).padStart(2,"0")).join(""),c=performance.now(),{hash:m,nonce:i}=await a({basePrefix:"/",version:"devel"},r,Number(t),o),u=performance.now();return console.log({hash:m,nonce:i}),e.time+=u-c,e.iters+=i,{time:u-c,nonce:i}},f={time:0,iters:0},b={time:0,iters:0},B=()=>{let e=f.iters/f.time,t=b.iters/b.time;if(Number.isFinite(e)){if(g.innerText=`Average hashrate: ${e.toFixed(3)}kH/s`,Number.isFinite(t)){let n=(e-t)/e*100;g.innerText+=` vs ${t.toFixed(3)}kH/s (${n.toFixed(2)}% change)`}}else g.innerText="Benchmarking..."},w=e=>{let t=document.createElement("td");return t.innerText=e,t.style.padding="0 0.25rem",t},$=async e=>{let t=k.value,n=S.value,o=v.value;B();try{let{time:a,nonce:l}=await I(f,t,n,e.signal),r=document.createElement("tr");r.style.display="contents",r.append(w(`${a}ms`),w(l));let c=s.scrollHeight-s.clientHeight<=s.scrollTop;if(s.append(r),c&&(s.scrollTop=s.scrollHeight-s.clientHeight),B(),o!=="NONE"){let{time:m,nonce:i}=await I(b,t,o,e.signal);r.append(w(`${m}ms`),w(i))}}catch(a){a!==!1&&(g.innerText=a);return}await $(e)},T=null,H=()=>{f.time=f.iters=0,b.time=b.iters=0,s.innerHTML=g.innerText="";let e=s.parentElement;v.value!=="NONE"?(e.style.gridTemplateColumns="repeat(4,auto)",A.style.display="none",P.style.display="contents"):(e.style.gridTemplateColumns="repeat(2,auto)",A.style.display="contents",P.style.display="none"),T?.abort(),T=new AbortController,$(T)};F();k.addEventListener("change",H);S.addEventListener("change",H);v.addEventListener("change",H);H();})();
//# sourceMappingURL=bench.mjs.map
