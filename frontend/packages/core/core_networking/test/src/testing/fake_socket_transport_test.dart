import 'package:core_networking/testing.dart';
import 'package:test/test.dart';

void main() {
  test('send records messages and invokes onSend', () {
    final received = <dynamic>[];
    final transport = FakeSocketTransport(onSend: received.add);
    transport.send('hello');
    expect(transport.sentMessages, ['hello']);
    expect(received, ['hello']);
  });

  test('emitMessage forwards through stream', () async {
    final transport = FakeSocketTransport();
    final messages = <dynamic>[];
    transport.stream.listen(messages.add);
    transport.emitMessage('hi');
    await Future<void>.delayed(Duration.zero);
    expect(messages, ['hi']);
  });

  test('emitError forwards an error through stream', () async {
    final transport = FakeSocketTransport();
    final errors = <Object>[];
    transport.stream.listen((_) {}, onError: errors.add);
    transport.emitError('boom');
    await Future<void>.delayed(Duration.zero);
    expect(errors, ['boom']);
  });

  test('emitDone closes the stream', () async {
    final transport = FakeSocketTransport();
    expect(transport.stream, emitsDone);
    await transport.emitDone();
  });

  test('close sets closed flag', () async {
    final transport = FakeSocketTransport();
    await transport.close();
    expect(transport.closed, isTrue);
  });

  test('close throws when failToClose is set', () async {
    final transport = FakeSocketTransport(failToClose: true);
    await expectLater(transport.close, throwsStateError);
    expect(transport.closed, isTrue);
  });

  test('close after emitDone does not throw (idempotent)', () async {
    final transport = FakeSocketTransport();
    await transport.emitDone();
    await transport.close();
    expect(transport.closed, isTrue);
  });
}
