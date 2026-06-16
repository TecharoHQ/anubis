use anubis::{DATA_BUFFER, DATA_LENGTH, update_nonce};
use argon2::Argon2;
use std::sync::{LazyLock, Mutex};

/// Argon2id result and verification hashes are 32 bytes (256 bits). These are stored in
/// static buffers due to the fact that you cannot easily pass data from host space to
/// WebAssembly space.
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

/// Computes the Argon2id hash for the given nonce.
///
/// The challenge bytes are used directly as the Argon2id password and the nonce is used as
/// the salt, rather than hashing a hex-formatted `challenge || nonce` string as the
/// JavaScript implementation does. Operating on the raw challenge bytes avoids the
/// formatting round-trip.
///
/// The nonce is also randomly encoded in either big or little endian depending on the last
/// byte of the data buffer in an effort to make it more annoying to automate with GPUs.
fn compute_hash(nonce: u32) -> [u8; 32] {
    let data = &DATA_BUFFER;
    let data_len = *DATA_LENGTH.lock().unwrap();
    let data_slice: &[u8] = &data[..data_len];
    let use_le = data_slice.last().copied().unwrap_or(0) >= 128;
    let mut result = [0u8; 32];

    let nonce = nonce as u64;

    let nonce = if use_le {
        nonce.to_le_bytes()
    } else {
        nonce.to_be_bytes()
    };

    let argon2 = Argon2::default();
    argon2
        .hash_password_into(&data_slice, &nonce, &mut result)
        .unwrap();
    result
}

/// This function is the main entrypoint for the Anubis proof of work implementation.
///
/// This expects `DATA_BUFFER` to be prepopulated with the challenge value as "raw bytes".
/// The definition of what goes in the data buffer is an exercise for the implementor, but
/// for Argon2id we store the challenge as "raw bytes". The data buffer is intentionally
/// oversized so that the challenge value can be expanded in the future.
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
    let mut nonce = initial_nonce;

    loop {
        let hash = compute_hash(nonce);

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
        // progress bar and hashrate. Comparing the shifted window (rather than the original
        // `nonce > old_nonce + 1023`) reports correctly for any `iterand` and stays correct
        // across the u32 wraparound. (Only the nonce-0 worker's callback is live; the caller
        // silences the rest.)
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
    let computed = compute_hash(nonce);
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
