import { Sha256 } from '@aws-crypto/sha256-js';

const sha256 = (text) => {
  const hash = new Sha256();
  hash.update(text);
  return hash.digest()
    .then((result) =>
      Array.from(new Uint8Array(result))
        .map((c) => c.toString(16).padStart(2, "0"))
        .join(""),
    );
};

addEventListener('message', async (event) => {
  let data = event.data.data;
  let difficulty = event.data.difficulty;

  let hash;
  let nonce = 0;
  do {
    if ((nonce & 1023) === 0) {
      postMessage(nonce);
    }
    hash = await sha256(data + nonce++);
  } while (hash.substring(0, difficulty) !== Array(difficulty + 1).join('0'));

  nonce -= 1; // last nonce was post-incremented

  postMessage({
    hash,
    data,
    difficulty,
    nonce,
  });
});
