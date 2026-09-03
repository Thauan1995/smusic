// Standard `flutter drive` entrypoint for the `integration_test` package -
// required boilerplate, not project-specific logic. See
// integration_test/README.md and frontend/README.md's "Testes E2E (Web,
// browser real)" section for how this is invoked (via chromedriver).
import 'package:integration_test/integration_test_driver.dart';

Future<void> main() => integrationDriver();
