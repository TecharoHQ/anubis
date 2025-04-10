use std::boxed::Box;
use std::sync::{LazyLock, Mutex};

#[cfg(target_arch = "wasm32")]
mod hostimport {
    use crate::{DATA_BUFFER, DATA_LENGTH};

    #[link(wasm_import_module = "anubis")]
    unsafe extern "C" {
        /// The runtime expects this function to be defined. It is called whenever the Anubis check
        /// worker processes about 1024 hashes. This can be a no-op if you want.
        fn anubis_update_nonce(nonce: u32);
    }

    /// Safe wrapper to `anubis_update_nonce`.
    pub fn update_nonce(nonce: u32) {
        unsafe {
            anubis_update_nonce(nonce);
        }
    }

    #[unsafe(no_mangle)]
    pub extern "C" fn data_ptr() -> *const u8 {
        let challenge = &DATA_BUFFER;
        challenge.as_ptr()
    }

    #[unsafe(no_mangle)]
    pub extern "C" fn set_data_length(len: u32) {
        let mut data_length = DATA_LENGTH.lock().unwrap();
        *data_length = len as usize;
    }
}

#[cfg(not(target_arch = "wasm32"))]
mod hostimport {
    pub fn update_nonce(_nonce: u32) {
        // This is intentionally blank
    }
}

/// The data buffer is a bit weird in that it doesn't have an explicit length as it can
/// and will change depending on the challenge input that was sent by the server.
/// However, it can only fit 4096 bytes of data (one amd64 machine page). This is
/// slightly overkill for the purposes of an Anubis check, but it's fine to assume
/// that the browser can afford this much ram usage.
///
/// Callers should fetch the base data pointer, write up to 4096 bytes, and then
/// `set_data_length` the number of bytes they have written
///
/// This is also functionally a write-only buffer, so it doesn't really matter that
/// the length of this buffer isn't exposed.
pub static DATA_BUFFER: LazyLock<[u8; 4096]> = LazyLock::new(|| [0; 4096]);
pub static DATA_LENGTH: LazyLock<Mutex<usize>> = LazyLock::new(|| Mutex::new(0));

pub use hostimport::update_nonce;
