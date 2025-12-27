#pragma once

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

// Derives a Juno Orchard-only unified address (`j1...`) from a Juno UFVK (`jview1...`) and a
// diversifier index.
//
// Returns a newly-allocated UTF-8 JSON string with one of:
//   - {"status":"ok","address":"j1..."}
//   - {"status":"err","error":"..."}
//
// The returned pointer must be freed with `juno_addrgen_string_free`.
char *juno_addrgen_derive_json(const char *ufvk_utf8, uint32_t index);

// Derives a batch of Juno Orchard-only unified addresses (`j1...`) from a Juno UFVK (`jview1...`).
//
// Returns a newly-allocated UTF-8 JSON string with one of:
//   - {"status":"ok","start":<u32>,"count":<u32>,"addresses":["j1...","j1..."]}
//   - {"status":"err","error":"..."}
//
// The returned pointer must be freed with `juno_addrgen_string_free`.
char *juno_addrgen_batch_json(const char *ufvk_utf8, uint32_t start, uint32_t count);

// Frees a string returned by `juno_addrgen_derive_json` / `juno_addrgen_batch_json`.
void juno_addrgen_string_free(char *s);

#ifdef __cplusplus
} // extern "C"
#endif

