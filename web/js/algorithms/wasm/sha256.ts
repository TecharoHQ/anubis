import {
  memory,
  data_ptr,
  set_data_length,
  anubis_work,
  anubis_validate,
  result_hash_ptr,
  result_hash_size,
  verification_hash_ptr,
  verification_hash_size,
} from "../../../static/wasm/baseline/sha256.mjs";

export default {
  memory,
  data_ptr,
  set_data_length,
  anubis_work,
  anubis_validate,
  result_hash_ptr,
  result_hash_size,
  verification_hash_ptr,
  verification_hash_size
};