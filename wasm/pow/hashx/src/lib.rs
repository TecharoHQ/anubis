use anubis::{DATA_BUFFER, DATA_LENGTH, update_nonce};
use hashx::HashX;
use std::sync::{LazyLock, Mutex};

/// HashX result and verification hashes are 32 bytes (256 bits). These are stored in static
/// buffers due to the fact that you cannot easily pass data from host space to WebAssembly
/// space.
pub static RESULT_HASH: LazyLock<Mutex<[u8; 32]>> = LazyLock::new(|| Mutex::new([0; 32]));

pub static VERIFICATION_HASH: LazyLock<Mutex<[u8; 32]>> = LazyLock::new(|| Mutex::new([0; 32]));

/// Core validation function. Compare each bit in the hash by progressively masking bits until
/// some are found to not be matching.
///
/// There are probably more clever ways to do this, likely involving lookup tables or something
/// really fun like that. However in my testing this lets us get up to 200 kilohashes per second
/// on my Ryzen 7950x3D, up from about 50 kilohashes per second in JavaScript.
fn validate(hash: &[u8], difficulty: u32) -> bool {
    let mut remaining = difficulty;
    for &byte in hash {
        // If we're out of bits to check, exit. This is all good.
        if remaining == 0 {
            break;
        }

        // If there are more than 8 bits remaining, the entire byte should be a
        // zero. This fast-path compares the byte to 0 and if it matches, subtract
        // 8 bits.
        if remaining >= 8 {
            if byte != 0 {
                return false;
            }
            remaining -= 8;
        } else {
            // Otherwise mask off individual bits and check against them.
            let mask = 0xFF << (8 - remaining);
            if (byte & mask) != 0 {
                return false;
            }
            remaining = 0;
        }
    }
    true
}

/// Build a `HashX` instance for the current challenge.
///
/// HashX rejects a small fraction of seeds (`Error::ProgramConstraints`). Rather than trap
/// on those, derive a usable instance deterministically: the overwhelmingly common case is
/// that the challenge bytes are accepted as-is, but if they are rejected we append an
/// incrementing little-endian counter to the challenge until `HashX::new` accepts the seed.
/// The client and server run this identical code, so they always agree on which seed — and
/// therefore which program — is in use. The probability of needing even one retry is well
/// under a percent, so the common path is unchanged on the wire.
fn make_hashx() -> HashX {
    let data = &DATA_BUFFER;
    let data_len = *DATA_LENGTH.lock().unwrap();
    let data_slice: &[u8] = &data[..data_len];

    if let Ok(h) = HashX::new(data_slice) {
        return h;
    }

    let mut seed = Vec::with_capacity(data_slice.len() + 4);
    let mut counter: u32 = 0;
    loop {
        seed.clear();
        seed.extend_from_slice(data_slice);
        seed.extend_from_slice(&counter.to_le_bytes());
        if let Ok(h) = HashX::new(&seed) {
            return h;
        }
        counter = counter.wrapping_add(1);
    }
}

/// This function is the main entrypoint for the Anubis proof of work implementation.
///
/// This expects `DATA_BUFFER` to be prepopulated with the challenge value as "raw bytes".
/// The definition of what goes in the data buffer is an exercise for the implementor, but
/// for SHA-256 we store the hash as "raw bytes". The data buffer is intentionally oversized
/// so that the challenge value can be expanded in the future.
///
/// `difficulty` is the number of leading bits that must match `0` in order for the
/// challenge to be successfully passed. This will be validated by the server.
///
/// `initial_nonce` is the initial value of the nonce (number used once). This nonce will be
/// appended to the challenge value in order to find a hash matching the specified
/// difficulty.
///
/// `iterand` (noun form of iterate) is the amount that the nonce should be increased by
/// every iteration of the proof of work loop. This will vary by how many threads are
/// running the proof-of-work check, and also functions as a thread ID. This prevents
/// wasting CPU time retrying a hash+nonce pair that likely won't work.
#[unsafe(no_mangle)]
pub extern "C" fn anubis_work(difficulty: u32, initial_nonce: u32, iterand: u32) -> u32 {
    let h = make_hashx();
    let mut nonce: u32 = initial_nonce;

    loop {
        let hash = h.hash_to_bytes(nonce as u64);

        if validate(&hash, difficulty) {
            // If the challenge worked, copy the bytes into `RESULT_HASH` so the runtime
            // can pick it up.
            let mut result = RESULT_HASH.lock().unwrap();
            result.copy_from_slice(&hash);
            return nonce;
        }

        let old_nonce = nonce;
        nonce = nonce.wrapping_add(iterand);

        // Report progress each time the search crosses into a new 1024-value window of the
        // nonce space, i.e. roughly every 1024 nonce values. The reported nonce approximates
        // the total work done across all worker threads, which the client turns into a
        // progress bar and hashrate. Comparing the shifted window reports correctly for any
        // `iterand` and stays correct across the u32 wraparound. (Only the nonce-0 worker's
        // callback is live; the caller silences the rest.)
        if (nonce >> 10) != (old_nonce >> 10) {
            update_nonce(nonce);
        }
    }
}

/// This function is called by the server in order to validate a proof-of-work challenge.
/// This expects `DATA_BUFFER` to be set to the challenge value and `VERIFICATION_HASH` to
/// be set to the "raw bytes" of the SHA-256 hash that the client calculated.
///
/// If everything is good, it returns true. Otherwise, it returns false.
///
/// XXX(Xe): this could probably return an error code for what step fails, but this is fine
/// for now.
#[unsafe(no_mangle)]
pub extern "C" fn anubis_validate(nonce: u32, difficulty: u32) -> bool {
    let h = make_hashx();
    let computed = h.hash_to_bytes(nonce as u64);
    let valid = validate(&computed, difficulty);
    if !valid {
        return false;
    }

    let verification = VERIFICATION_HASH.lock().unwrap();
    computed == *verification
}

// These functions exist to give pointers and lengths to the runtime around the Anubis
// checks, this allows JavaScript and Go to safely manipulate the memory layout that Rust
// has statically allocated at compile time without having to assume how the Rust compiler
// is going to lay it out.

#[unsafe(no_mangle)]
pub extern "C" fn result_hash_ptr() -> *const u8 {
    let challenge = RESULT_HASH.lock().unwrap();
    challenge.as_ptr()
}

#[unsafe(no_mangle)]
pub extern "C" fn result_hash_size() -> usize {
    RESULT_HASH.lock().unwrap().len()
}

#[unsafe(no_mangle)]
pub extern "C" fn verification_hash_ptr() -> *const u8 {
    let verification = VERIFICATION_HASH.lock().unwrap();
    verification.as_ptr()
}

#[unsafe(no_mangle)]
pub extern "C" fn verification_hash_size() -> usize {
    VERIFICATION_HASH.lock().unwrap().len()
}
