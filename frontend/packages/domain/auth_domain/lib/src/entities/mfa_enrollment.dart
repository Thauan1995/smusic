/// `POST /v1/auth/mfa/enroll`'s response - a fresh TOTP secret plus the
/// `otpauth://` URI an authenticator app (Google Authenticator, Authy,
/// etc.) can consume. [secret] is shown alongside [otpauthUrl] since this
/// slice has no QR-code renderer - a user types [secret] into their
/// authenticator manually.
class MfaEnrollment {
  const MfaEnrollment({required this.secret, required this.otpauthUrl});

  final String secret;
  final String otpauthUrl;
}
