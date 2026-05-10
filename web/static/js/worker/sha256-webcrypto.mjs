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
(()=>{var h=new TextEncoder,y=async e=>{let s=h.encode(e);return await crypto.subtle.digest("SHA-256",s)},g=e=>e.reduce((s,a)=>s+a.toString(16).padStart(2,"0"),"");addEventListener("message",async({data:e})=>{let{data:s,difficulty:a,threads:d}=e,t=e.nonce,f=t===0,o=0,c=Math.floor(a/2),l=a%2!==0;for(;;){let u=await y(s+t),i=new Uint8Array(u),r=!0;for(let n=0;n<c;n++)if(i[n]!==0){r=!1;break}if(r&&l&&i[c]>>4!==0&&(r=!1),r){let n=g(i);postMessage({hash:n,data:s,difficulty:a,nonce:t});return}t+=d,o++,t%1!==0&&(t=Math.trunc(t)),f&&(o&1023)===0&&postMessage(t)}});})();
//# sourceMappingURL=sha256-webcrypto.mjs.map
