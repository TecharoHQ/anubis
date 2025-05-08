import { Sha256 } from '@aws-crypto/sha256-js';

const sha256 = (text) => {
  const hash = new Sha256();
  hash.update(text);
  return hash.digest();
};

function uint8ArrayToHexString(arr) {
  return Array.from(arr)
    .map((c) => c.toString(16).padStart(2, "0"))
    .join("");
}

addEventListener('message', async (event) => {
  let data = event.data.data;
  let difficulty = event.data.difficulty;
  let hash;
  let nonce = event.data.nonce;
  let threads = event.data.threads;

  const threadId = nonce;

  while (true) {
    const currentHash = await sha256(data + nonce);
    const thisHash = new Uint8Array(currentHash);
    let valid = true;

    for (let j = 0; j < difficulty; j++) {
      const byteIndex = Math.floor(j / 2); // which byte we are looking at
      const nibbleIndex = j % 2; // which nibble in the byte we are looking at (0 is high, 1 is low)

      let nibble = (thisHash[byteIndex] >> (nibbleIndex === 0 ? 4 : 0)) & 0x0F; // Get the nibble

      if (nibble !== 0) {
        valid = false;
        break;
      }
    }

    if (valid) {
      hash = uint8ArrayToHexString(thisHash);
      break;
    }

    const oldNonce = nonce;
    nonce += threads;

    // send a progress update every 1024 iterations. since each thread checks
    // separate values, one simple way to do this is by bit masking the
    // nonce for multiples of 1024. unfortunately, if the number of threads
    // is not prime, only some of the threads will be sending the status
    // update and they will get behind the others. this is slightly more
    // complicated but ensures an even distribution between threads.
    if (
      nonce > oldNonce | 1023 && // we've wrapped past 1024
      (nonce >> 10) % threads === threadId // and it's our turn
    ) {
      postMessage(nonce);
    }
  }

  postMessage({
    hash,
    data,
    difficulty,
    nonce,
  });
});