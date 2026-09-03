import 'package:flutter/material.dart';

/// The one primary call-to-action button style used across auth/library/
/// player screens (sign in, play, retry). A thin wrapper over
/// `FilledButton` so every screen gets the same shape/elevation/loading
/// affordance without repeating it.
class SmusicPrimaryButton extends StatelessWidget {
  const SmusicPrimaryButton({
    super.key,
    required this.label,
    required this.onPressed,
    this.isLoading = false,
  });

  final String label;
  final VoidCallback? onPressed;
  final bool isLoading;

  @override
  Widget build(BuildContext context) {
    return FilledButton(
      onPressed: isLoading ? null : onPressed,
      child: isLoading
          ? const SizedBox(
              width: 18,
              height: 18,
              child: CircularProgressIndicator(strokeWidth: 2),
            )
          : Text(label),
    );
  }
}
