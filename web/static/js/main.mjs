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
(()=>{var I=()=>navigator.hardwareConcurrency!==void 0?navigator.hardwareConcurrency:1;function _(e,n,s=5,o=null,i,u=Math.trunc(Math.max(I()/2,1))){console.debug("fast algo");let a="purejs";return window.isSecureContext&&(a="webcrypto"),(navigator.userAgent.includes("Firefox")||navigator.userAgent.includes("Goanna"))&&(console.log("Firefox detected, using pure-JS fallback"),a="purejs"),new Promise((E,x)=>{let M=`${e.basePrefix}/.within.website/x/cmd/anubis/static/js/worker/sha256-${a}.mjs?cacheBuster=${e.version}`,p=[],d=!1,b=()=>{console.log("PoW aborted"),h(),x(new DOMException("Aborted","AbortError"))},h=()=>{d||(d=!0,p.forEach(c=>c.terminate()),o?.removeEventListener("abort",b))};if(o!=null){if(o.aborted)return b();o.addEventListener("abort",b,{once:!0})}for(let c=0;c<u;c++){let g=new Worker(M);g.onmessage=m=>{typeof m.data=="number"?i?.(m.data):(h(),E(m.data))},g.onerror=m=>{h(),x(m)},g.postMessage({data:n,difficulty:s,nonce:c,threads:u}),p.push(g)}})}var j={fast:_,slow:_};var v=(e="",n={})=>{let s=new URL(e,window.location.href);return Object.entries(n).forEach(([o,i])=>s.searchParams.set(o,i)),s.toString()},L=e=>{let n=document.getElementById(e);return n===null?null:JSON.parse(n.textContent)},k=(e,n,s)=>v(`${s}/.within.website/x/cmd/anubis/static/img/${e}.webp`,{cacheBuster:n});var W=async()=>document.documentElement.lang,S=async e=>{let n=L("anubis_base_prefix");if(n!==null)try{return await(await fetch(`${n}/.within.website/x/cmd/anubis/static/locales/${e}.json`)).json()}catch(s){if(console.warn(`Failed to load translations for ${e}, falling back to English`),e!=="en")return await S("en");throw s}},C=()=>{let e=L("anubis_public_url");if(e!==null)return e&&window.location.href.startsWith(e)?new URLSearchParams(window.location.search).get("redir"):window.location.href},$={},D,A=async()=>{D=await W(),$=await S(D)},r=e=>$[`js_${e}`]||$[e]||e;(async()=>{await A();let e=[{name:"Web Workers",msg:r("web_workers_error"),value:window.Worker},{name:"Cookies",msg:r("cookies_error"),value:navigator.cookieEnabled}],n=document.getElementById("status"),s=document.getElementById("image"),o=document.getElementById("title"),i=document.getElementById("progress"),u=L("anubis_version"),a=L("anubis_base_prefix"),E=document.querySelector("details"),x=!1;E&&E.addEventListener("toggle",()=>{E.open&&(x=!0)});let M=({titleMsg:l,statusMsg:f,imageSrc:w})=>{o.innerHTML=l,n.innerHTML=f,s.src=w,i.style.display="none"};n.innerHTML=r("calculating");for(let{value:l,name:f,msg:w}of e)if(!l){M({titleMsg:`${r("missing_feature")} ${f}`,statusMsg:w,imageSrc:k("reject",u,a)});return}let{challenge:p,rules:d}=L("anubis_challenge"),b=j[d.algorithm];if(!b){M({titleMsg:r("challenge_error"),statusMsg:r("challenge_error_msg"),imageSrc:k("reject",u,a)});return}n.innerHTML=`${r("calculating_difficulty")} ${d.difficulty}, `,i.style.display="inline-block";let h=document.createTextNode(`${r("speed")} 0kH/s`);n.appendChild(h);let c=0,g=!1,m=Math.pow(16,-d.difficulty);try{let l=Date.now(),{hash:f,nonce:w}=await b({basePrefix:a,version:u},p.randomData,d.difficulty,null,t=>{let y=Date.now()-l;y-c>1e3&&(c=y,h.data=`${r("speed")} ${(t/y).toFixed(3)}kH/s`);let T=Math.pow(1-m,t),P=(1-Math.pow(T,2))*100;i["aria-valuenow"]=P,i.firstElementChild!==null&&(i.firstElementChild.style.width=`${P}%`),T<.1&&!g&&(n.append(document.createElement("br"),document.createTextNode(r("verification_longer"))),g=!0)}),H=Date.now();if(console.log({hash:f,nonce:w}),x){let y=function(){let T=C();window.location.replace(v(`${a}/.within.website/x/cmd/anubis/api/pass-challenge`,{id:p.id,response:f,nonce:w,redir:T,elapsedTime:H-l}))},t=document.getElementById("progress");t.style.display="flex",t.style.alignItems="center",t.style.justifyContent="center",t.style.height="2rem",t.style.borderRadius="1rem",t.style.cursor="pointer",t.style.background="#b16286",t.style.color="white",t.style.fontWeight="bold",t.style.outline="4px solid #b16286",t.style.outlineOffset="2px",t.style.width="min(20rem, 90%)",t.style.margin="1rem auto 2rem",t.innerHTML=r("finished_reading"),t.onclick=y,setTimeout(y,3e4)}else{let t=C();window.location.replace(v(`${a}/.within.website/x/cmd/anubis/api/pass-challenge`,{id:p.id,response:f,nonce:w,redir:t,elapsedTime:H-l}))}}catch(l){M({titleMsg:r("calculation_error"),statusMsg:`${r("calculation_error_msg")} ${l.message}`,imageSrc:k("reject",u,a)})}})();})();
//# sourceMappingURL=main.mjs.map
