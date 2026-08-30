/*
@licstart  The following is the entire license notice for the
JavaScript code in this page.

Copyright (c) 2026 Xe Iaso <xe.iaso@techaro.lol>

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

@licend  The above is the entire license notice
for the JavaScript code in this page.

## Rationale

This script catches `main.mjs` failing to load and re-injects it with
jittered exponential backoff.

Without this any failures loading `main.mjs` leaves the challenge page
sitting on the "Loading..." message forever with no error messaging
outside of the browser console, which is unavailable on mobile browsers.

Disable loading this script at your peril. You have been warned.
*/
(()=>{var f=(e,t=750,n=1e4)=>Math.random()*Math.min(n,t*Math.pow(e,2));var d=(e,t={},n=[])=>{let s=typeof e=="function"?e(t):Object.assign(document.createElement(e),t);return Array.isArray(n)||(n=[n]),s.append(...n),s};var i=e=>document.getElementById(e);var _=e=>e.map(i);(()=>{let e=i("anubis-main");if(!e){console.debug("can't find anubis main.mjs element via ID `anubis-main`, bailing");return}let t=e.getAttribute("src");if(!t){console.debug("can't find src attribute of script element `anubis-main`, bailing");return}let n=t.indexOf("?")===-1?"?":"&",s=4,E=750,A=1e4,c=1e4,a=0,l=0,u=()=>window.__anubisBooted===!0,p=()=>{let r=_(["anubis-script-error","status","progress"]);if(r.filter(D=>D==null).length!==0){console.debug("missing one of the following elements: anubis-script-error, status, progress. cannot proceed, bailing.");return}let[o,M,T]=r;o.style.display="block",M.style.display="none",T.style.display="none"},m=r=>()=>{if(u()||r!==l)return;if(l++,a++,a>s){p();return}let o=f(a,E,A);setTimeout(y,o)},y=()=>{if(u())return;let r=m(l),o=d("script",{async:!0,type:"module",src:t+n+"anubisRetry="+a,onerror:r});document.head.appendChild(o),setTimeout(r,c)},b=m(0);e.onerror=b,setTimeout(b,c)})();})();
