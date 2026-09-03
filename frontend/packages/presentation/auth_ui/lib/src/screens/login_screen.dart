import 'package:auth_domain/auth_domain.dart';
import 'package:core_design_system/core_design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../auth_error_messages.dart';

/// Login screen (task scope item 3). Never navigates itself - `onLoggedIn`
/// is called once `authSessionProvider` transitions to a signed-in state,
/// so `shared_navigation` (which owns `go_router`) decides what "logged
/// in" navigates to. Keeps `auth_ui` free of a `go_router` dependency.
class LoginScreen extends HookConsumerWidget {
  const LoginScreen({super.key, this.onLoggedIn, this.onNavigateToSignUp});

  final VoidCallback? onLoggedIn;
  final VoidCallback? onNavigateToSignUp;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final formKey = useMemoized(() => GlobalKey<FormState>());
    final emailController = useTextEditingController();
    final passwordController = useTextEditingController();

    ref.listen<AsyncValue<AuthUser?>>(authSessionProvider, (previous, next) {
      final wasSignedIn = previous?.value != null;
      final isSignedIn = next.value != null;
      if (!wasSignedIn && isSignedIn) onLoggedIn?.call();
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
                      'smusic',
                      style: Theme.of(context).textTheme.headlineMedium,
                      textAlign: TextAlign.center,
                    ),
                    const SizedBox(height: SmusicSpacing.lg),
                    TextFormField(
                      key: const Key('login_email_field'),
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
                      key: const Key('login_password_field'),
                      controller: passwordController,
                      decoration: const InputDecoration(labelText: 'Password'),
                      obscureText: true,
                      autofillHints: const [AutofillHints.password],
                      validator: (value) => (value == null || value.length < 8)
                          ? 'Password must be at least 8 characters'
                          : null,
                    ),
                    if (errorMessage != null) ...[
                      const SizedBox(height: SmusicSpacing.sm),
                      Text(
                        errorMessage,
                        key: const Key('login_error_text'),
                        style: TextStyle(color: Theme.of(context).colorScheme.error),
                      ),
                    ],
                    const SizedBox(height: SmusicSpacing.lg),
                    SmusicPrimaryButton(
                      label: 'Log in',
                      isLoading: authState.isLoading,
                      onPressed: () {
                        if (formKey.currentState?.validate() != true) return;
                        ref.read(authSessionProvider.notifier).logIn(
                              email: emailController.text.trim(),
                              password: passwordController.text,
                            );
                      },
                    ),
                    const SizedBox(height: SmusicSpacing.md),
                    TextButton(
                      onPressed: onNavigateToSignUp,
                      child: const Text("Don't have an account? Sign up"),
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
