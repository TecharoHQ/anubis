// Load and instantiate the .wasm file
const response = await fetch("sha256.wasm");

const importObject = {
  anubis: {
    anubis_update_nonce: (nonce) => {
      console.log(`Received nonce update: ${nonce}`);
      // Your logic here
    }
  }
};

const module = await WebAssembly.compileStreaming(response);
const instance = await WebAssembly.instantiate(module, importObject);

// Get exports
const {
  anubis_work,
  anubis_validate,
  data_ptr,
  result_hash_ptr,
  result_hash_size,
  verification_hash_ptr,
  verification_hash_size,
  set_data_length,
  memory
} = instance.exports;

console.log(instance.exports);

function uint8ArrayToHex(arr) {
  return Array.from(arr)
    .map((c) => c.toString(16).padStart(2, "0"))
    .join("");
}

function hexToUint8Array(hexString) {
  // Remove whitespace and optional '0x' prefix
  hexString = hexString.replace(/\s+/g, '').replace(/^0x/, '');

  // Check for valid length
  if (hexString.length % 2 !== 0) {
    throw new Error('Invalid hex string length');
  }

  // Check for valid characters
  if (!/^[0-9a-fA-F]+$/.test(hexString)) {
    throw new Error('Invalid hex characters');
  }

  // Convert to Uint8Array
  const byteArray = new Uint8Array(hexString.length / 2);
  for (let i = 0; i < byteArray.length; i++) {
    const byteValue = parseInt(hexString.substr(i * 2, 2), 16);
    byteArray[i] = byteValue;
  }

  return byteArray;
}

// Write data to buffer
function writeToBuffer(data) {
  if (data.length > 1024) throw new Error("Data exceeds buffer size");

  // Get pointer and create view
  const offset = data_ptr();
  const buffer = new Uint8Array(memory.buffer, offset, data.length);

  // Copy data
  buffer.set(data);

  // Set data length
  set_data_length(data.length);
}

function readFromChallenge() {
  const offset = result_hash_ptr();
  const buffer = new Uint8Array(memory.buffer, offset, result_hash_size());

  return buffer;
}

// Example usage:
const data = hexToUint8Array("98ea6e4f216f2fb4b69fff9b3a44842c38686ca685f3f55dc48c5d3fb1107be4");
writeToBuffer(data);

// Call work function
const t0 = Date.now();
const nonce = anubis_work(16, 0, 1);
const t1 = Date.now();

console.log(`Done! Took ${t1 - t0}ms, ${nonce} iterations`);

const challengeBuffer = readFromChallenge();

{
  const buffer = new Uint8Array(memory.buffer, verification_hash_ptr(), verification_hash_size());
  buffer.set(challengeBuffer);
}

// Validate
const isValid = anubis_validate(nonce, 10) === 1;
console.log(isValid);

console.log(uint8ArrayToHex(readFromChallenge()));