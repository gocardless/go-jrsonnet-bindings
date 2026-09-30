//! Strings passed in from Go.

use std::{borrow::Cow, ffi::OsStr, marker::PhantomData, os::unix::ffi::OsStrExt, path::Path};

/// A Go string with the layout of cgo's `_GoString_`, so Go passes a
/// `string` without copying it. It may contain NUL and invalid UTF-8.
///
/// Only Go creates these. The bytes stay valid for the duration of the call,
/// and the lifetime `'a` makes the compiler reject keeping them longer. Rust
/// copies whatever it keeps.
#[repr(C)]
pub struct GoStr<'a> {
    p: *const u8,
    n: isize,
    _go: PhantomData<&'a [u8]>,
}

impl<'a> GoStr<'a> {
    pub(crate) fn as_bytes(&self) -> &'a [u8] {
        match usize::try_from(self.n) {
            Ok(len) if len > 0 && !self.p.is_null() => {
                // SAFETY: Go passes `n` readable bytes at `p`, valid for the call.
                unsafe { std::slice::from_raw_parts(self.p, len) }
            }
            _ => &[],
        }
    }

    /// The string, if it is valid UTF-8.
    pub(crate) fn as_str(&self) -> Option<&str> {
        std::str::from_utf8(self.as_bytes()).ok()
    }

    /// The string as go-jsonnet sees it. Like Go's `string([]rune(s))`, each
    /// invalid byte becomes one U+FFFD. `String::from_utf8_lossy` differs,
    /// as it replaces a truncated multi-byte sequence with a single U+FFFD.
    pub(crate) fn as_str_lossy(&self) -> Cow<'_, str> {
        decode_like_go(self.as_bytes())
    }

    /// The string as a path, byte for byte.
    pub(crate) fn as_path(&self) -> &Path {
        path_from_bytes(self.as_bytes())
    }
}

/// A path, byte for byte.
pub(crate) fn path_from_bytes(bytes: &[u8]) -> &Path {
    Path::new(OsStr::from_bytes(bytes))
}

/// Decodes `bytes` like [`GoStr::as_str_lossy`].
pub(crate) fn decode_like_go(bytes: &[u8]) -> Cow<'_, str> {
    if let Ok(s) = std::str::from_utf8(bytes) {
        return Cow::Borrowed(s);
    }
    let mut out = String::with_capacity(bytes.len() + 8);
    for chunk in bytes.utf8_chunks() {
        out.push_str(chunk.valid());
        for _ in chunk.invalid() {
            out.push(char::REPLACEMENT_CHARACTER);
        }
    }
    Cow::Owned(out)
}

#[cfg(test)]
mod tests {
    use super::decode_like_go;

    #[test]
    fn decodes_like_go() {
        // Expected values are Go's string([]rune(s)).
        let cases: &[(&[u8], &str)] = &[
            (b"", ""),
            (b"plain", "plain"),
            ("h\u{e9}llo \u{2603}".as_bytes(), "h\u{e9}llo \u{2603}"),
            (b"a\x00b", "a\u{0}b"),
            (b"\xff\xfe", "\u{fffd}\u{fffd}"),
            (b"\xe2\x82A", "\u{fffd}\u{fffd}A"),
            (b"a\xffb\xe2\x82c", "a\u{fffd}b\u{fffd}\u{fffd}c"),
            (b"\xf0\x9f\x98", "\u{fffd}\u{fffd}\u{fffd}"),
            (b"\xed\xa0\x80", "\u{fffd}\u{fffd}\u{fffd}"), // surrogate
            (b"\xc0\x80", "\u{fffd}\u{fffd}"),             // overlong NUL
        ];
        for (input, want) in cases {
            assert_eq!(decode_like_go(input), *want, "input {input:?}");
        }
    }
}
