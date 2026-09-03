import 'package:auth_domain/auth_domain.dart';
import 'package:core_design_system/core_design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../auth_error_messages.dart';

/// Sign-up screen (task scope item 3). Same navigation-decoupling
/// rationale as [LoginScreen] - see its doc comment.
class SignUpScreen extends HookConsumerWidget {
  const SignUpScreen({super.key, this.onSignedUp, this.onNavigateToLogin});

  final VoidCallback? onSignedUp;
  final VoidCallback? onNavigateToLogin;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final formKey = useMemoized(() => GlobalKey<FormState>());
    final displayNameController = useTextEditingController();
    final emailController = useTextEditingController();
    final passwordController = useTextEditingController();

    ref.listen<AsyncValue<AuthUser?>>(authSessionProvider, (previous, next) {
      final wasSignedIn = previous?.value != null;
      final isSignedIn = next.value != null;
      if (!wasSignedIn && isSignedIn) onSignedUp?.call();
    });

    final authState = ref.watch(authSessionProvider);
    final errorMessage = authState.hasError ? authErrorMessage(authState.error) : null;

    return Scaffold(
      body: SafeArea(
        child: Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 400),
            child: SingleChildScrollView(
              padding: const EdgeInsets.all(SmusicSpacing.lg),
              child: Form(
                key: formKey,
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    Text(
                      'Create your account',
                      style: Theme.of(context).textTheme.headlineMedium,
                      textAlign: TextAlign.center,
                    ),
                    const SizedBox(height: SmusicSpacing.lg),
                    TextFormField(
                      key: const Key('signup_display_name_field'),
                      controller: displayNameController,
                      decoration: const InputDecoration(labelText: 'Display name'),
                      validator: (value) =>
                          (value == null || value.trim().isEmpty) ? 'Enter a name' : null,
                    ),
                    const SizedBox(height: SmusicSpacing.md),
                    TextFormField(
                      key: const Key('signup_email_field'),
                      controller: emailController,
                      decoration: const InputDecoration(labelText: 'Email'),
                      keyboardType: TextInputType.emailAddress,
                      autofillHints: const [AutofillHints.email],
                      validator: (value) => (value == null || !value.contains('@'))
                          ? 'Enter a valid email'
                          : null,
                    ),
                    const SizedBox(height: SmusicSpacing.md),
                    TextFormField(
                      key: const Key('signup_password_field'),
                      controller: passwordController,
                      decoration: const InputDecoration(labelText: 'Password'),
                      obscureText: true,
                      autofillHints: const [AutofillHints.newPassword],
                      validator: (value) => (value == null || value.length < 8)
                          ? 'Password must be at least 8 characters'
                          : null,
                    ),
                    if (errorMessage != null) ...[
                      const SizedBox(height: SmusicSpacing.sm),
                      Text(
                        errorMessage,
                        key: const Key('signup_error_text'),
                        style: TextStyle(color: Theme.of(context).colorScheme.error),
                      ),
                    ],
                    const SizedBox(height: SmusicSpacing.lg),
                    SmusicPrimaryButton(
                      label: 'Sign up',
                      isLoading: authState.isLoading,
                      onPressed: () {
                        if (formKey.currentState?.validate() != true) return;
                        ref.read(authSessionProvider.notifier).signUp(
                              email: emailController.text.trim(),
                              password: passwordController.text,
                              displayName: displayNameController.text.trim(),
                            );
                      },
                    ),
                    const SizedBox(height: SmusicSpacing.md),
                    TextButton(
                      onPressed: onNavigateToLogin,
                      child: const Text('Already have an account? Log in'),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
