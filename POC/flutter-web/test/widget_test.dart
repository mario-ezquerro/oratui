import 'package:flutter_test/flutter_test.dart';
import 'package:oratui_web/main.dart';

void main() {
  testWidgets('ORA-TUI Web Command Center renders title', (WidgetTester tester) async {
    await tester.pumpWidget(const OraTuiCommandCenterApp());
    expect(find.textContaining('ORA-TUI'), findsWidgets);
  });
}
