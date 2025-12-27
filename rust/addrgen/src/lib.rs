use core::ffi::c_char;

use orchard::keys::{FullViewingKey, Scope};
use serde::Serialize;

mod zip316;

const HRP_JUNO_UFVK: &str = "jview";
const HRP_JUNO_UA: &str = "j";
const TYPECODE_ORCHARD: u64 = 0x03;

const MAX_BATCH_COUNT: u32 = 100_000;

#[derive(Clone, Copy, Debug)]
enum ErrorCode {
    UfvkEmpty,
    UfvkInvalidBech32m,
    UfvkHrpMismatch,
    UfvkTlvInvalid,
    UfvkTypecodeUnsupported,
    UfvkValueLenInvalid,
    UfvkFvkBytesInvalid,
    CountZero,
    CountTooLarge,
    RangeOverflow,
    Internal,
}

impl ErrorCode {
    fn as_str(self) -> &'static str {
        match self {
            ErrorCode::UfvkEmpty => "ufvk_empty",
            ErrorCode::UfvkInvalidBech32m => "ufvk_invalid_bech32m",
            ErrorCode::UfvkHrpMismatch => "ufvk_hrp_mismatch",
            ErrorCode::UfvkTlvInvalid => "ufvk_tlv_invalid",
            ErrorCode::UfvkTypecodeUnsupported => "ufvk_typecode_unsupported",
            ErrorCode::UfvkValueLenInvalid => "ufvk_value_len_invalid",
            ErrorCode::UfvkFvkBytesInvalid => "ufvk_fvk_bytes_invalid",
            ErrorCode::CountZero => "count_zero",
            ErrorCode::CountTooLarge => "count_too_large",
            ErrorCode::RangeOverflow => "range_overflow",
            ErrorCode::Internal => "internal",
        }
    }
}

fn decode_fvk_from_ufvk(ufvk: &str) -> Result<FullViewingKey, ErrorCode> {
    let ufvk = ufvk.trim();
    if ufvk.is_empty() {
        return Err(ErrorCode::UfvkEmpty);
    }

    let (typecode, value) = zip316::decode_single_tlv_container(HRP_JUNO_UFVK, ufvk)
        .map_err(map_zip316_err)?;

    if typecode != TYPECODE_ORCHARD {
        return Err(ErrorCode::UfvkTypecodeUnsupported);
    }
    if value.len() != 96 {
        return Err(ErrorCode::UfvkValueLenInvalid);
    }
    let fvk_bytes: [u8; 96] = value.try_into().map_err(|_| ErrorCode::UfvkValueLenInvalid)?;

    FullViewingKey::from_bytes(&fvk_bytes).ok_or(ErrorCode::UfvkFvkBytesInvalid)
}

fn derive_address_from_fvk(fvk: &FullViewingKey, index: u32) -> Result<String, ErrorCode> {
    let addr = fvk.address_at(index, Scope::External);
    let raw = addr.to_raw_address_bytes();
    zip316::encode_unified_container(HRP_JUNO_UA, TYPECODE_ORCHARD, &raw)
        .map_err(|_| ErrorCode::Internal)
}

fn derive_address_from_ufvk(ufvk: &str, index: u32) -> Result<String, ErrorCode> {
    let fvk = decode_fvk_from_ufvk(ufvk)?;
    derive_address_from_fvk(&fvk, index)
}

fn derive_addresses_from_ufvk(
    ufvk: &str,
    start: u32,
    count: u32,
) -> Result<Vec<String>, ErrorCode> {
    if count == 0 {
        return Err(ErrorCode::CountZero);
    }
    if count > MAX_BATCH_COUNT {
        return Err(ErrorCode::CountTooLarge);
    }

    let end_exclusive = start.checked_add(count).ok_or(ErrorCode::RangeOverflow)?;

    let fvk = decode_fvk_from_ufvk(ufvk)?;
    let mut out = Vec::with_capacity(count as usize);
    for index in start..end_exclusive {
        out.push(derive_address_from_fvk(&fvk, index)?);
    }
    Ok(out)
}

fn map_zip316_err(e: zip316::Zip316Error) -> ErrorCode {
    use zip316::Zip316Error;
    match e {
        Zip316Error::Bech32DecodeFailed
        | Zip316Error::InvalidHrp
        | Zip316Error::PayloadTooShort
        | Zip316Error::PaddingInvalid
        | Zip316Error::F4JumbleFailed => ErrorCode::UfvkInvalidBech32m,
        Zip316Error::HrpMismatch => ErrorCode::UfvkHrpMismatch,
        Zip316Error::TlvInvalid | Zip316Error::TlvTrailingBytes => ErrorCode::UfvkTlvInvalid,
        Zip316Error::HrpTooLong | Zip316Error::Bech32EncodeFailed => ErrorCode::Internal,
    }
}

#[derive(Serialize)]
#[serde(tag = "status", rename_all = "snake_case")]
enum DeriveResponse {
    Ok { address: String },
    Err { error: String },
}

#[derive(Serialize)]
#[serde(tag = "status", rename_all = "snake_case")]
enum BatchResponse {
    Ok {
        start: u32,
        count: u32,
        addresses: Vec<String>,
    },
    Err { error: String },
}

fn to_c_string<T: Serialize>(v: &T) -> *mut c_char {
    let json = serde_json::to_string(v)
        .unwrap_or_else(|_| r#"{"status":"err","error":"internal"}"#.to_string());
    // JSON contains no interior NULs.
    std::ffi::CString::new(json).expect("json").into_raw()
}

#[no_mangle]
pub extern "C" fn juno_addrgen_derive_json(ufvk_utf8: *const c_char, index: u32) -> *mut c_char {
    let res = std::panic::catch_unwind(|| {
        if ufvk_utf8.is_null() {
            return DeriveResponse::Err {
                error: ErrorCode::UfvkEmpty.as_str().to_string(),
            };
        }

        let ufvk = unsafe { std::ffi::CStr::from_ptr(ufvk_utf8) }.to_string_lossy();
        match derive_address_from_ufvk(&ufvk, index) {
            Ok(address) => DeriveResponse::Ok { address },
            Err(code) => DeriveResponse::Err {
                error: code.as_str().to_string(),
            },
        }
    });

    match res {
        Ok(v) => to_c_string(&v),
        Err(_) => to_c_string(&DeriveResponse::Err {
            error: ErrorCode::Internal.as_str().to_string(),
        }),
    }
}

#[no_mangle]
pub extern "C" fn juno_addrgen_batch_json(
    ufvk_utf8: *const c_char,
    start: u32,
    count: u32,
) -> *mut c_char {
    let res = std::panic::catch_unwind(|| {
        if ufvk_utf8.is_null() {
            return BatchResponse::Err {
                error: ErrorCode::UfvkEmpty.as_str().to_string(),
            };
        }

        let ufvk = unsafe { std::ffi::CStr::from_ptr(ufvk_utf8) }.to_string_lossy();
        match derive_addresses_from_ufvk(&ufvk, start, count) {
            Ok(addresses) => BatchResponse::Ok {
                start,
                count,
                addresses,
            },
            Err(code) => BatchResponse::Err {
                error: code.as_str().to_string(),
            },
        }
    });

    match res {
        Ok(v) => to_c_string(&v),
        Err(_) => to_c_string(&BatchResponse::Err {
            error: ErrorCode::Internal.as_str().to_string(),
        }),
    }
}

#[no_mangle]
pub extern "C" fn juno_addrgen_string_free(s: *mut c_char) {
    if s.is_null() {
        return;
    }
    unsafe {
        drop(std::ffi::CString::from_raw(s));
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    use zip32::AccountId;

    const JUNO_COIN_TYPE: u32 = 8133;

    #[test]
    fn derives_address_from_generated_ufvk() {
        let seed = [7u8; 64];
        let account = AccountId::try_from(0).expect("account");
        let sk =
            orchard::keys::SpendingKey::from_zip32_seed(&seed, JUNO_COIN_TYPE, account).expect("sk");
        let fvk = FullViewingKey::from(&sk);

        let ufvk =
            zip316::encode_unified_container(HRP_JUNO_UFVK, TYPECODE_ORCHARD, &fvk.to_bytes())
                .expect("ufvk");

        let got = derive_address_from_ufvk(&ufvk, 0).expect("addr");

        let expected_raw = fvk.address_at(0u32, Scope::External).to_raw_address_bytes();
        let expected =
            zip316::encode_unified_container(HRP_JUNO_UA, TYPECODE_ORCHARD, &expected_raw)
                .expect("expected");

        assert_eq!(got, expected);
    }

    #[test]
    fn batch_matches_single_derivation() {
        let seed = [9u8; 64];
        let account = AccountId::try_from(0).expect("account");
        let sk =
            orchard::keys::SpendingKey::from_zip32_seed(&seed, JUNO_COIN_TYPE, account).expect("sk");
        let fvk = FullViewingKey::from(&sk);

        let ufvk =
            zip316::encode_unified_container(HRP_JUNO_UFVK, TYPECODE_ORCHARD, &fvk.to_bytes())
                .expect("ufvk");

        let single = derive_address_from_ufvk(&ufvk, 5).expect("single");
        let batch = derive_addresses_from_ufvk(&ufvk, 5, 1).expect("batch");
        assert_eq!(batch, vec![single]);
    }
}
