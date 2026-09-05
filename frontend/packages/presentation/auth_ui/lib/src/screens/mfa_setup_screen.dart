import 'package:auth_domain/auth_domain.dart';
import 'package:core_design_system/core_design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../auth_error_messages.dart';

/// Two-factor (TOTP) enrollment - the step-up auth security.md §2 requires
/// before `ProximityPrivacySettingsNotifier.enableFeature` can succeed
/// (`ProximityExceptionKind.mfaRequired`). Reached by pushing this screen
/// and popping `true` once [verify] confirms the code, so the caller (the
/// proximity settings screen) knows to retry enabling the feature - see
/// this screen's `onVerified` callback, mirroring `LoginScreen`'s
/// never-navigate-itself convention.
///
/// No QR-code renderer ships in this slice (no dependency on one exists
/// yet in `auth_ui`) - the secret is shown as selectable text the user
/// copies into their authenticator app manually, alongside the full
/// `otpauth://` URL for authenticator apps that accept pasting one.
class MfaSetupScreen extends HookConsumerWidget {
  const MfaSetupScreen({super.key, this.onVerified});

  final VoidCallback? onVerified;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final formKey = useMemoized(() => GlobalKey<FormState>());
    final codeController = useTextEditingController();
    final enrollmentState = ref.watch(mfaEnrollmentProvider);
    final isVerifying = useState(false);
    final verifyError = useState<String?>(null);

    useEffect(() {
      Future.microtask(() => ref.read(mfaEnrollmentProvider.notifier).enroll());
      return null;
    }, const []);

    Future<void> handleVerify() async {
      if (formKey.currentState?.validate() != true) return;
      isVerifying.value = true;
      verifyError.value = null;
      try {
        await ref.read(mfaEnrollmentProvider.notifier).verify(codeController.text.trim());
        onVerified?.call();
      } on AuthException catch (e) {
        verifyError.value = authErrorMessage(e);
      } finally {
        isVerifying.value = false;
      }
    }

    return Scaffold(
      appBar: AppBar(title: const Text('Set up two-factor authentication')),
      body: SafeArea(
        child: Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 400),
            child: SingleChildScrollView(
              padding: const EdgeInsets.all(SmusicSpacing.lg),
              child: enrollmentState.when(
                loading: () => const Center(child: CircularProgressIndicator()),
                error: (error, stackTrace) => EmptyState(
                  icon: Icons.error_outline,
                  message: 'Could not start enrollment. Please try again.',
                  actionLabel: 'Retry',
                  onAction: () => ref.read(mfaEnrollmentProvider.notifier).enroll(),
                ),
                data: (enrollment) {
                  if (enrollment == null) {
                    return const SizedBox.shrink();
                  }
                  return Form(
                    key: formKey,
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        const Text(
                          'Nearby discovery requires a second factor to keep your '
                          'location sharing secure. Add this key to an authenticator '
                          'app (Google Authenticator, Authy, etc.), then enter the '
                          '6-digit code it shows.',
                        ),
                        const SizedBox(height: SmusicSpacing.lg),
                        _SecretField(secret: enrollment.secret),
                        const SizedBox(height: SmusicSpacing.md),
                        TextFormField(
                          key: const Key('mfa_code_field'),
                          controller: codeController,
                          decoration: const InputDecoration(labelText: '6-digit code'),
                          keyboardType: TextInputType.number,
                          maxLength: 6,
                          validator: (value) => (value == null || value.trim().length != 6)
                              ? 'Enter the 6-digit code from your authenticator app'
                              : null,
                        ),
                        if (verifyError.value != null) ...[
                          const SizedBox(height: SmusicSpacing.sm),
                          Text(
                            verifyError.value!,
                            key: const Key('mfa_verify_error_text'),
                            style: TextStyle(color: Theme.of(context).colorScheme.error),
                          ),
                        ],
                        const SizedBox(height: SmusicSpacing.lg),
                        SmusicPrimaryButton(
                          label: 'Verify and enable',
                          isLoading: isVerifying.value,
                          onPressed: handleVerify,
                        ),
                      ],
                    ),
                  );
                },
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _SecretField extends StatelessWidget {
  const _SecretField({required this.secret});

  final String secret;

  @override
  Widget build(BuildContext context) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(SmusicSpacing.md),
        child: Row(
          children: [
            Expanded(
              child: SelectableText(
                secret,
                key: const Key('mfa_secret_text'),
                style: Theme.of(context)
                    .textTheme
                    .titleMedium
                    ?.copyWith(fontFeatures: const [FontFeature.tabularFigures()]),
              ),
            ),
            IconButton(
              key: const Key('mfa_secret_copy_button'),
              icon: const Icon(Icons.copy),
              tooltip: 'Copy',
              onPressed: () => Clipboard.setData(ClipboardData(text: secret)),
            ),
          ],
        ),
      ),
    );
  }
}
