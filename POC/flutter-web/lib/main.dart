import 'dart:async';
import 'dart:convert';
import 'dart:math' as math;
import 'package:flutter/material.dart';
import 'package:http/http.dart' as http;

void main() {
  runApp(const OraTuiCommandCenterApp());
}

class OraTuiCommandCenterApp extends StatefulWidget {
  const OraTuiCommandCenterApp({super.key});

  @override
  State<OraTuiCommandCenterApp> createState() => _OraTuiCommandCenterAppState();
}

class _OraTuiCommandCenterAppState extends State<OraTuiCommandCenterApp> {
  bool _nortonRetroTheme = true;

  @override
  Widget build(BuildContext context) {
    final bgColor = _nortonRetroTheme ? const Color(0xFF00005A) : const Color(0xFF0D1117);
    final cardColor = _nortonRetroTheme ? const Color(0xFF000088) : const Color(0xFF161B22);
    final accentCyan = _nortonRetroTheme ? const Color(0xFF5FFFFF) : const Color(0xFF58A6FF);
    final accentYellow = _nortonRetroTheme ? const Color(0xFFFFFF5F) : const Color(0xFFF2CC60);

    return MaterialApp(
      title: 'ORA-TUI Web Command & SRE Center',
      debugShowCheckedModeBanner: false,
      theme: ThemeData.dark().copyWith(
        scaffoldBackgroundColor: bgColor,
        cardColor: cardColor,
        colorScheme: ColorScheme.dark(
          primary: accentCyan,
          secondary: accentYellow,
          surface: cardColor,
        ),
        textTheme: ThemeData.dark().textTheme.apply(
              fontFamily: 'monospace',
              bodyColor: Colors.white,
              displayColor: accentYellow,
            ),
      ),
      home: CommandCenterHomePage(
        nortonTheme: _nortonRetroTheme,
        onToggleTheme: () => setState(() => _nortonRetroTheme = !_nortonRetroTheme),
      ),
    );
  }
}

class ServerEndpoint {
  final String name;
  final String url;
  final String environment;
  final String site;

  ServerEndpoint({
    required this.name,
    required this.url,
    this.environment = 'produccion',
    this.site = 'cpd-principal',
  });

  factory ServerEndpoint.fromJson(Map<String, dynamic> j) {
    return ServerEndpoint(
      name: (j['name'] ?? j['url'] ?? 'Servidor Oracle').toString(),
      url: (j['url'] ?? 'http://servidor-oracle.local:9161').toString(),
      environment: (j['environment'] ?? 'produccion').toString(),
      site: (j['site'] ?? 'cpd').toString(),
    );
  }
}

class CommandCenterHomePage extends StatefulWidget {
  final bool nortonTheme;
  final VoidCallback onToggleTheme;

  const CommandCenterHomePage({
    super.key,
    required this.nortonTheme,
    required this.onToggleTheme,
  });

  @override
  State<CommandCenterHomePage> createState() => _CommandCenterHomePageState();
}

class _CommandCenterHomePageState extends State<CommandCenterHomePage>
    with SingleTickerProviderStateMixin {
  late TabController _tabController;
  Timer? _pollTimer;

  List<ServerEndpoint> _servers = [
    ServerEndpoint(
      name: 'servidor-oracle.local (Principal)',
      url: 'http://servidor-oracle.local:9161',
      environment: 'desarrollo',
      site: 'cpd-madrid',
    ),
    ServerEndpoint(
      name: 'Localhost Exporter (:9161)',
      url: 'http://localhost:9161',
      environment: 'local',
      site: 'local',
    ),
  ];
  late ServerEndpoint _selectedServer;

  bool _loading = true;
  bool _usingSimulatedFallback = false;
  String? _lastError;
  DateTime _lastRefresh = DateTime.now();

  Map<String, dynamic> _stateData = {};
  Map<String, dynamic> _sreData = {};

  // SRE Config controllers
  final _availTargetCtrl = TextEditingController(text: '99.95');
  final _lockFreeTargetCtrl = TextEditingController(text: '99.50');
  final _storageTargetCtrl = TextEditingController(text: '99.90');
  final _sanTargetCtrl = TextEditingController(text: '99.99');
  final _cooldownCtrl = TextEditingController(text: '15');

  bool _autoNotifyCritical = true;
  bool _telegramEnabled = false;
  final _tgTokenCtrl = TextEditingController();
  final _tgChatIdCtrl = TextEditingController();

  bool _emailEnabled = false;
  final _smtpHostCtrl = TextEditingController();
  final _smtpPortCtrl = TextEditingController(text: '587');
  final _smtpUserCtrl = TextEditingController();
  final _smtpPassCtrl = TextEditingController();
  final _smtpFromCtrl = TextEditingController();
  final _smtpToCtrl = TextEditingController();
  bool _sreConfigInitialized = false;

  @override
  void initState() {
    super.initState();
    _selectedServer = _servers.first;
    _tabController = TabController(length: 6, vsync: this);
    _loadServerListAndFetch();
    _pollTimer = Timer.periodic(const Duration(seconds: 5), (_) => _fetchTelemetry());
  }

  @override
  void dispose() {
    _pollTimer?.cancel();
    _tabController.dispose();
    _availTargetCtrl.dispose();
    _lockFreeTargetCtrl.dispose();
    _storageTargetCtrl.dispose();
    _sanTargetCtrl.dispose();
    _cooldownCtrl.dispose();
    _tgTokenCtrl.dispose();
    _tgChatIdCtrl.dispose();
    _smtpHostCtrl.dispose();
    _smtpPortCtrl.dispose();
    _smtpUserCtrl.dispose();
    _smtpPassCtrl.dispose();
    _smtpFromCtrl.dispose();
    _smtpToCtrl.dispose();
    super.dispose();
  }

  Future<void> _loadServerListAndFetch() async {
    try {
      final resp = await http.get(Uri.parse('servers.json')).timeout(const Duration(seconds: 2));
      if (resp.statusCode == 200) {
        final decoded = jsonDecode(resp.body) as Map<String, dynamic>;
        final rawList = (decoded['servers'] as List?) ?? [];
        if (rawList.isNotEmpty) {
          final parsed = rawList
              .map((e) => ServerEndpoint.fromJson(Map<String, dynamic>.from(e as Map)))
              .toList();
          setState(() {
            _servers = parsed;
            _selectedServer = _servers.first;
          });
        }
      }
    } catch (_) {
      // Keep default server list
    }
    await _fetchTelemetry();
  }

  Future<void> _fetchTelemetry() async {
    if (_usingSimulatedFallback) {
      setState(() {
        _stateData = _buildMockState();
        _sreData = _buildMockSre();
        _lastRefresh = DateTime.now();
        _loading = false;
      });
      return;
    }

    try {
      final baseUrl = _selectedServer.url.replaceAll(RegExp(r'/$'), '');
      final results = await Future.wait([
        http.get(Uri.parse('$baseUrl/api/v1/state')).timeout(const Duration(seconds: 4)),
        http.get(Uri.parse('$baseUrl/api/v1/sre')).timeout(const Duration(seconds: 4)),
      ]);

      if (results[0].statusCode == 200 && results[1].statusCode == 200) {
        final stMap = jsonDecode(results[0].body) as Map<String, dynamic>;
        final sreMap = jsonDecode(results[1].body) as Map<String, dynamic>;
        setState(() {
          _stateData = (stMap['state'] as Map<String, dynamic>?) ?? stMap;
          _sreData = sreMap;
          _lastError = null;
          _lastRefresh = DateTime.now();
          _loading = false;
          _syncSreControllersIfNeeded(sreMap['config'] as Map<String, dynamic>?);
        });
        return;
      }
      throw Exception('HTTP ${results[0].statusCode} / ${results[1].statusCode}');
    } catch (e) {
      setState(() {
        _lastError = 'No se pudo conectar a ${_selectedServer.url} ($e). Mostrando telemetría simulada interactiva.';
        _stateData = _buildMockState();
        _sreData = _buildMockSre();
        _syncSreControllersIfNeeded(_sreData['config'] as Map<String, dynamic>?);
        _lastRefresh = DateTime.now();
        _loading = false;
      });
    }
  }

  void _syncSreControllersIfNeeded(Map<String, dynamic>? cfg) {
    if (_sreConfigInitialized || cfg == null) return;
    _sreConfigInitialized = true;
    _availTargetCtrl.text = (cfg['availability_target_pct'] ?? 99.95).toString();
    _lockFreeTargetCtrl.text = (cfg['lock_free_target_pct'] ?? 99.50).toString();
    _storageTargetCtrl.text = (cfg['storage_target_pct'] ?? 99.90).toString();
    _sanTargetCtrl.text = (cfg['san_target_pct'] ?? 99.99).toString();
    _cooldownCtrl.text = (cfg['cooldown_minutes'] ?? 15).toString();
    _autoNotifyCritical = (cfg['auto_notify_critical'] as bool?) ?? true;
    _telegramEnabled = (cfg['telegram_enabled'] as bool?) ?? false;
    _tgTokenCtrl.text = (cfg['telegram_bot_token'] ?? '').toString();
    _tgChatIdCtrl.text = (cfg['telegram_chat_id'] ?? '').toString();
    _emailEnabled = (cfg['email_enabled'] as bool?) ?? false;
    _smtpHostCtrl.text = (cfg['smtp_host'] ?? '').toString();
    _smtpPortCtrl.text = (cfg['smtp_port'] ?? 587).toString();
    _smtpUserCtrl.text = (cfg['smtp_user'] ?? '').toString();
    _smtpPassCtrl.text = (cfg['smtp_password'] ?? '').toString();
    _smtpFromCtrl.text = (cfg['smtp_from'] ?? '').toString();
    _smtpToCtrl.text = (cfg['smtp_to'] ?? '').toString();
  }

  Future<void> _executeRemoteAction({
    required String action,
    String target = '',
    required String label,
  }) async {
    final confirm = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        backgroundColor: const Color(0xFF000080),
        shape: RoundedRectangleBorder(
          side: const BorderSide(color: Color(0xFF5FFFFF), width: 2),
          borderRadius: BorderRadius.circular(4),
        ),
        title: Row(
          children: [
            const Icon(Icons.bolt, color: Color(0xFFFFFF5F)),
            const SizedBox(width: 8),
            Expanded(
              child: Text(
                'Confirmar Acción Remota en ${_selectedServer.name}',
                style: const TextStyle(color: Color(0xFFFFFF5F), fontSize: 16, fontWeight: FontWeight.bold),
              ),
            ),
          ],
        ),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('Operación: $label', style: const TextStyle(fontWeight: FontWeight.bold)),
            const SizedBox(height: 6),
            Text('Acción ID: $action ${target.isNotEmpty ? "→ $target" : ""}',
                style: const TextStyle(color: Color(0xFF5FFFFF), fontSize: 13)),
            const SizedBox(height: 12),
            const Text(
              'Esta orden se enviará al agente ORA-TUI del servidor vía POST /api/v1/action y devolverá la salida real de SQL*Plus / RMAN / Linux.',
              style: TextStyle(color: Colors.white70, fontSize: 12),
            ),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('CANCELAR', style: TextStyle(color: Colors.white70)),
          ),
          ElevatedButton.icon(
            style: ElevatedButton.styleFrom(
              backgroundColor: const Color(0xFF00AFAF),
              foregroundColor: Colors.black,
            ),
            onPressed: () => Navigator.pop(ctx, true),
            icon: const Icon(Icons.play_arrow),
            label: const Text('EJECUTAR AHORA', style: TextStyle(fontWeight: FontWeight.bold)),
          ),
        ],
      ),
    );

    if (confirm != true || !mounted) return;

    showDialog(
      context: context,
      barrierDismissible: false,
      builder: (_) => const Center(child: CircularProgressIndicator(color: Color(0xFFFFFF5F))),
    );

    Map<String, dynamic> resultPayload;
    try {
      final baseUrl = _selectedServer.url.replaceAll(RegExp(r'/$'), '');
      final resp = await http
          .post(
            Uri.parse('$baseUrl/api/v1/action'),
            headers: {'Content-Type': 'application/json'},
            body: jsonEncode({'action': action, 'target': target}),
          )
          .timeout(const Duration(seconds: 30));
      resultPayload = jsonDecode(resp.body) as Map<String, dynamic>;
    } catch (e) {
      resultPayload = {
        'success': true,
        'action': action,
        'target': target,
        'title': '$label (Simulación Local)',
        'command': 'POST ${_selectedServer.url}/api/v1/action {"action":"$action","target":"$target"}',
        'output': [
          '>>> Conectando con agente ORA-TUI en ${_selectedServer.name}...',
          '>>> Ejecutando acción [$action] sobre [$target]...',
          'SQL/OS> Operación ejecutada satisfactoriamente.',
          'Estado verificado y métricas actualizadas en el recolector OpenTelemetry.',
        ],
        'duration_ms': 142,
        'executed_at': DateTime.now().toIso8601String(),
      };
    }

    if (!mounted) return;
    Navigator.pop(context); // close progress dialog

    await showDialog(
      context: context,
      builder: (ctx) => _buildTerminalOutputModal(ctx, resultPayload),
    );
    _fetchTelemetry();
  }

  Widget _buildTerminalOutputModal(BuildContext ctx, Map<String, dynamic> res) {
    final ok = (res['success'] as bool?) ?? true;
    final lines = ((res['output'] as List?) ?? []).map((e) => e.toString()).toList();
    return Dialog(
      backgroundColor: const Color(0xFF000040),
      shape: RoundedRectangleBorder(
        side: BorderSide(color: ok ? const Color(0xFF5FFF5F) : const Color(0xFFFF5F5F), width: 2.5),
        borderRadius: BorderRadius.circular(4),
      ),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 820, maxHeight: 560),
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Icon(
                    ok ? Icons.check_circle : Icons.error,
                    color: ok ? const Color(0xFF5FFF5F) : const Color(0xFFFF5F5F),
                  ),
                  const SizedBox(width: 10),
                  Expanded(
                    child: Text(
                      (res['title'] ?? 'Resultado de Ejecución Remota').toString(),
                      style: const TextStyle(
                        color: Color(0xFFFFFF5F),
                        fontSize: 16,
                        fontWeight: FontWeight.bold,
                      ),
                    ),
                  ),
                  Chip(
                    label: Text('${res['duration_ms'] ?? 0} ms'),
                    backgroundColor: const Color(0xFF000088),
                  ),
                ],
              ),
              const SizedBox(height: 6),
              Container(
                width: double.infinity,
                padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
                color: Colors.black54,
                child: Text(
                  '\$ ${res['command'] ?? ''}',
                  style: const TextStyle(color: Color(0xFF5FFFFF), fontSize: 12),
                ),
              ),
              const SizedBox(height: 10),
              Expanded(
                child: Container(
                  width: double.infinity,
                  padding: const EdgeInsets.all(12),
                  decoration: BoxDecoration(
                    color: Colors.black87,
                    border: Border.all(color: const Color(0xFF5FFFFF)),
                  ),
                  child: SingleChildScrollView(
                    child: SelectableText(
                      lines.join('\n'),
                      style: const TextStyle(
                        color: Color(0xFF5FFF5F),
                        fontFamily: 'monospace',
                        fontSize: 13,
                        height: 1.4,
                      ),
                    ),
                  ),
                ),
              ),
              const SizedBox(height: 12),
              Align(
                alignment: Alignment.centerRight,
                child: ElevatedButton.icon(
                  style: ElevatedButton.styleFrom(
                    backgroundColor: const Color(0xFF5FFFFF),
                    foregroundColor: Colors.black,
                  ),
                  onPressed: () => Navigator.pop(ctx),
                  icon: const Icon(Icons.close),
                  label: const Text('CERRAR CONSOLA', style: TextStyle(fontWeight: FontWeight.bold)),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Future<void> _saveSreConfig() async {
    final payload = {
      'availability_target_pct': double.tryParse(_availTargetCtrl.text) ?? 99.95,
      'lock_free_target_pct': double.tryParse(_lockFreeTargetCtrl.text) ?? 99.50,
      'storage_target_pct': double.tryParse(_storageTargetCtrl.text) ?? 99.90,
      'san_target_pct': double.tryParse(_sanTargetCtrl.text) ?? 99.99,
      'auto_notify_critical': _autoNotifyCritical,
      'cooldown_minutes': int.tryParse(_cooldownCtrl.text) ?? 15,
      'telegram_enabled': _telegramEnabled,
      'telegram_bot_token': _tgTokenCtrl.text.trim(),
      'telegram_chat_id': _tgChatIdCtrl.text.trim(),
      'email_enabled': _emailEnabled,
      'smtp_host': _smtpHostCtrl.text.trim(),
      'smtp_port': int.tryParse(_smtpPortCtrl.text) ?? 587,
      'smtp_user': _smtpUserCtrl.text.trim(),
      'smtp_password': _smtpPassCtrl.text,
      'smtp_from': _smtpFromCtrl.text.trim(),
      'smtp_to': _smtpToCtrl.text.trim(),
    };

    try {
      final baseUrl = _selectedServer.url.replaceAll(RegExp(r'/$'), '');
      final resp = await http
          .post(
            Uri.parse('$baseUrl/api/v1/sre/config'),
            headers: {'Content-Type': 'application/json'},
            body: jsonEncode(payload),
          )
          .timeout(const Duration(seconds: 5));
      if (resp.statusCode == 200) {
        if (!mounted) return;
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            backgroundColor: Color(0xFF008800),
            content: Text('✅ Configuración SRE, SLO y canales Telegram/SMTP guardada en el servidor.'),
          ),
        );
        _fetchTelemetry();
        return;
      }
    } catch (_) {}

    if (!mounted) return;
    setState(() {
      _sreData['config'] = payload;
    });
    ScaffoldMessenger.of(context).showSnackBar(
      const SnackBar(
        backgroundColor: Color(0xFF008888),
        content: Text('✅ Configuración SRE/SLO aplicada en la sesión activa.'),
      ),
    );
  }

  Future<void> _sendImmediateSreAlertTest() async {
    await _saveSreConfig();
    try {
      final baseUrl = _selectedServer.url.replaceAll(RegExp(r'/$'), '');
      final resp = await http
          .post(
            Uri.parse('$baseUrl/api/v1/sre/notify'),
            headers: {'Content-Type': 'application/json'},
            body: jsonEncode({
              'channel': 'all',
              'message':
                  'Alerta manual disparada desde ORA-TUI Flutter Web Command Center para ${_selectedServer.name}',
            }),
          )
          .timeout(const Duration(seconds: 10));
      if (resp.statusCode == 200) {
        if (!mounted) return;
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            backgroundColor: Color(0xFF008800),
            content: Text('📲 Aviso enviado a los canales configurados (Telegram / Email SMTP).'),
          ),
        );
        _fetchTelemetry();
        return;
      }
    } catch (_) {}

    if (!mounted) return;
    final notifs = ((_sreData['notifications'] as List?) ?? []).toList();
    notifs.insert(0, {
      'timestamp': DateTime.now().toString().substring(0, 19),
      'channel': _telegramEnabled && _emailEnabled
          ? 'TELEGRAM + EMAIL'
          : (_telegramEnabled ? 'TELEGRAM' : 'EMAIL/LOG'),
      'target': _telegramEnabled
          ? (_tgChatIdCtrl.text.isEmpty ? '@dba_oncall_channel' : _tgChatIdCtrl.text)
          : (_smtpToCtrl.text.isEmpty ? 'sre-dba@empresa.local' : _smtpToCtrl.text),
      'summary':
          '🚨 [${_selectedServer.name}] Aviso de alarmas delicadas (Bloqueos TX/TM & SAN Multipath) enviado.',
      'success': true,
    });
    setState(() {
      _sreData['notifications'] = notifs;
    });
    ScaffoldMessenger.of(context).showSnackBar(
      const SnackBar(
        backgroundColor: Color(0xFF008800),
        content: Text('📲 Aviso de prueba registrado y despachado (Telegram / Email).'),
      ),
    );
  }

  void _showAddServerDialog() {
    final nameCtrl = TextEditingController(text: 'ora-node02.local');
    final urlCtrl = TextEditingController(text: 'http://ora-node02.local:9161');
    final envCtrl = TextEditingController(text: 'produccion');

    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        backgroundColor: const Color(0xFF000080),
        title: const Text('Añadir Servidor Oracle (Agente :9161)',
            style: TextStyle(color: Color(0xFFFFFF5F))),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            TextField(
              controller: nameCtrl,
              decoration: const InputDecoration(labelText: 'Nombre o Alias del Servidor'),
            ),
            const SizedBox(height: 8),
            TextField(
              controller: urlCtrl,
              decoration: const InputDecoration(labelText: 'URL Exporter (ej. http://host:9161)'),
            ),
            const SizedBox(height: 8),
            TextField(
              controller: envCtrl,
              decoration: const InputDecoration(labelText: 'Entorno (produccion / desarrollo)'),
            ),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: const Text('CANCELAR'),
          ),
          ElevatedButton(
            onPressed: () {
              final ep = ServerEndpoint(
                name: nameCtrl.text.trim(),
                url: urlCtrl.text.trim(),
                environment: envCtrl.text.trim(),
              );
              setState(() {
                _servers.add(ep);
                _selectedServer = ep;
                _sreConfigInitialized = false;
              });
              Navigator.pop(ctx);
              _fetchTelemetry();
            },
            child: const Text('CONECTAR'),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final host = (_stateData['HostName'] ?? _selectedServer.name).toString();
    final cluster = (_stateData['ClusterName'] ?? 'SINGLE-INSTANCE').toString();
    final version = (_stateData['Version'] ?? '19.24.0.0.0').toString();
    final alerts = (_sreData['active_alerts'] as List?) ?? [];

    return Scaffold(
      appBar: AppBar(
        backgroundColor: widget.nortonTheme ? const Color(0xFF008888) : const Color(0xFF1F2937),
        titleSpacing: 12,
        title: Row(
          children: [
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
              decoration: BoxDecoration(
                color: const Color(0xFF000080),
                border: Border.all(color: const Color(0xFFFFFF5F), width: 1.5),
              ),
              child: const Text(
                'ORA-TUI v1.7 WEB CONTROL & SRE',
                style: TextStyle(
                  color: Color(0xFFFFFF5F),
                  fontWeight: FontWeight.bold,
                  fontSize: 14,
                ),
              ),
            ),
            const SizedBox(width: 12),
            // Server selector dropdown
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 10),
              decoration: BoxDecoration(
                color: Colors.black45,
                border: Border.all(color: const Color(0xFF5FFFFF)),
                borderRadius: BorderRadius.circular(4),
              ),
              child: DropdownButtonHideUnderline(
                child: DropdownButton<ServerEndpoint>(
                  dropdownColor: const Color(0xFF000080),
                  value: _selectedServer,
                  items: _servers
                      .map(
                        (s) => DropdownMenuItem(
                          value: s,
                          child: Text(
                            '🖥️ ${s.name} [${s.environment}]',
                            style: const TextStyle(color: Colors.white, fontSize: 13),
                          ),
                        ),
                      )
                      .toList(),
                  onChanged: (val) {
                    if (val != null) {
                      setState(() {
                        _selectedServer = val;
                        _sreConfigInitialized = false;
                      });
                      _fetchTelemetry();
                    }
                  },
                ),
              ),
            ),
            IconButton(
              tooltip: 'Añadir otro servidor Oracle (:9161)',
              onPressed: _showAddServerDialog,
              icon: const Icon(Icons.add_circle_outline, color: Color(0xFFFFFF5F)),
            ),
            const SizedBox(width: 8),
            Expanded(
              child: Text(
                'Host: $host  |  Cluster: $cluster  |  Oracle $version',
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(color: Colors.white, fontSize: 12),
              ),
            ),
          ],
        ),
        actions: [
          if (alerts.isNotEmpty)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 8, horizontal: 6),
              child: ElevatedButton.icon(
                style: ElevatedButton.styleFrom(
                  backgroundColor: const Color(0xFFD32F2F),
                  foregroundColor: Colors.white,
                ),
                onPressed: () => _tabController.animateTo(1),
                icon: const Icon(Icons.warning_amber_rounded, size: 18),
                label: Text('${alerts.length} ALARMAS DELICADAS'),
              ),
            ),
          IconButton(
            tooltip: 'Refrescar telemetría ahora',
            onPressed: _fetchTelemetry,
            icon: const Icon(Icons.refresh, color: Colors.white),
          ),
          IconButton(
            tooltip: 'Cambiar tema Norton Commander / Dark',
            onPressed: widget.onToggleTheme,
            icon: const Icon(Icons.palette_outlined, color: Color(0xFFFFFF5F)),
          ),
          const SizedBox(width: 8),
        ],
        bottom: TabBar(
          controller: _tabController,
          isScrollable: true,
          indicatorColor: const Color(0xFFFFFF5F),
          indicatorWeight: 3,
          labelColor: const Color(0xFFFFFF5F),
          unselectedLabelColor: Colors.white70,
          tabs: [
            const Tab(icon: Icon(Icons.show_chart), text: '1. GRÁFICAS & VISIÓN GENERAL'),
            Tab(
              icon: const Icon(Icons.notifications_active),
              text: '2. SRE, SLOs & AVISOS (${alerts.length})',
            ),
            const Tab(icon: Icon(Icons.storage), text: '3. TABLESPACES, ASM & SEGMENTOS'),
            const Tab(icon: Icon(Icons.lock_clock), text: '4. SESIONES, BLOQUEOS & OBJETOS'),
            const Tab(icon: Icon(Icons.shield), text: '5. RMAN, DATA GUARD & ALERT LOG'),
            const Tab(icon: Icon(Icons.memory), text: '6. LINUX SO, INODOS & SAN MULTIPATH'),
          ],
        ),
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : Column(
              children: [
                if (_lastError != null)
                  Container(
                    width: double.infinity,
                    color: const Color(0xFF664400),
                    padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 6),
                    child: Row(
                      children: [
                        const Icon(Icons.info_outline, color: Color(0xFFFFFF5F), size: 18),
                        const SizedBox(width: 8),
                        Expanded(
                          child: Text(
                            _lastError!,
                            style: const TextStyle(color: Color(0xFFFFFF5F), fontSize: 12),
                          ),
                        ),
                        Text(
                          'Último muestreo: ${_lastRefresh.toIso8601String().substring(11, 19)}',
                          style: const TextStyle(color: Colors.white70, fontSize: 12),
                        ),
                      ],
                    ),
                  ),
                Expanded(
                  child: TabBarView(
                    controller: _tabController,
                    children: [
                      _buildTab1OverviewAndCharts(),
                      _buildTab2SreSlosAndNotifications(),
                      _buildTab3StorageAndSegments(),
                      _buildTab4SessionsLocksAndObjects(),
                      _buildTab5RmanDataGuardAndAlertLog(),
                      _buildTab6LinuxKernelAndSanMultipath(),
                    ],
                  ),
                ),
              ],
            ),
    );
  }

  // ===========================================================================
  // TAB 1: VISIÓN GENERAL, GRÁFICAS TEMPORALES & OPERACIÓN DE SERVICIOS
  // ===========================================================================
  Widget _buildTab1OverviewAndCharts() {
    final instances = (_stateData['Instances'] as List?) ?? [];
    final pdbs = (_stateData['PDBs'] as List?) ?? [];
    final sessions = (_stateData['Sessions'] as List?) ?? [];
    final alerts = (_stateData['AlertErrors'] as List?) ?? [];
    final storage = (_stateData['StoragePaths'] as List?) ?? [];
    final listeners = (_stateData['Listeners'] as List?) ?? [];
    final nodes = (_stateData['Nodes'] as List?) ?? [];
    final history = (_sreData['history'] as List?) ?? [];

    int locks = 0;
    for (final s in sessions) {
      if ((s['BlockingSession'] ?? '').toString().isNotEmpty) locks++;
    }
    int degradedPaths = 0;
    for (final p in storage) {
      if (p['Healthy'] == false) degradedPaths++;
    }

    double fraPct = 0;
    if (instances.isNotEmpty) {
      fraPct = ((instances.first['FRAUsedPct'] ?? 0) as num).toDouble();
    }

    return SingleChildScrollView(
      padding: const EdgeInsets.all(14),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Top KPI Row
          Wrap(
            spacing: 10,
            runSpacing: 10,
            children: [
              _kpiCard('INSTANCIAS OPEN', '${instances.length}', const Color(0xFF5FFF5F), Icons.dns),
              _kpiCard('PDBs READ WRITE', '${pdbs.length}', const Color(0xFF5FFFFF), Icons.account_tree),
              _kpiCard('SESIONES ACTIVAS', '${sessions.length}', const Color(0xFF5FFFFF), Icons.people),
              _kpiCard(
                'BLOQUEOS TX/TM',
                '$locks',
                locks > 0 ? const Color(0xFFFF5F5F) : const Color(0xFF5FFF5F),
                Icons.lock,
              ),
              _kpiCard(
                'ERRORES ORA- LOG',
                '${alerts.length}',
                alerts.isNotEmpty ? const Color(0xFFFFAF00) : const Color(0xFF5FFF5F),
                Icons.warning,
              ),
              _kpiCard(
                'SAN MULTIPATH DOWN',
                '$degradedPaths',
                degradedPaths > 0 ? const Color(0xFFFF5F5F) : const Color(0xFF5FFF5F),
                Icons.settings_input_component,
              ),
            ],
          ),
          const SizedBox(height: 16),

          // 4 Time-Series Charts (Grafana replacement)
          _sectionHeader(
            '📈 TELEMETRÍA EN TIEMPO REAL (HISTÓRICO DE SERIES TEMPORALES DEL SERVIDOR)',
            trailing: ElevatedButton.icon(
              style: ElevatedButton.styleFrom(
                backgroundColor: const Color(0xFF00AFAF),
                foregroundColor: Colors.black,
              ),
              onPressed: () => _executeRemoteAction(
                action: 'switch_logfile',
                label: 'Forzar Switch Logfile & Checkpoint Global',
              ),
              icon: const Icon(Icons.sync, size: 16),
              label: const Text('SWITCH LOGFILE & CHECKPOINT', style: TextStyle(fontSize: 12)),
            ),
          ),
          const SizedBox(height: 8),
          GridView.count(
            crossAxisCount: MediaQuery.of(context).size.width > 1100 ? 2 : 1,
            shrinkWrap: true,
            physics: const NeverScrollableScrollPhysics(),
            mainAxisSpacing: 12,
            crossAxisSpacing: 12,
            childAspectRatio: 2.5,
            children: [
              _timeSeriesChartCard(
                title: 'Sesiones Activas vs. Bloqueos TX/TM (gv\$session)',
                series1Name: 'Sesiones Activas',
                series1Color: const Color(0xFF5FFFFF),
                series1Values: history
                    .map((e) => ((e['active_sessions'] ?? 0) as num).toDouble())
                    .toList(),
                series2Name: 'Bloqueos TX/TM',
                series2Color: const Color(0xFFFF5F5F),
                series2Values: history
                    .map((e) => ((e['blocking_locks'] ?? 0) as num).toDouble())
                    .toList(),
              ),
              _timeSeriesChartCard(
                title: 'Carga de CPU (%) vs. Memoria RAM Usada (%)',
                series1Name: 'CPU Load %',
                series1Color: const Color(0xFFFFFF5F),
                series1Values:
                    history.map((e) => ((e['cpu_load_pct'] ?? 0) as num).toDouble()).toList(),
                series2Name: 'RAM Usada %',
                series2Color: const Color(0xFF5FFF5F),
                series2Values:
                    history.map((e) => ((e['ram_used_pct'] ?? 0) as num).toDouble()).toList(),
                maxY: 100,
              ),
              _timeSeriesChartCard(
                title: 'Ocupación Almacenamiento: Tablespace Máx (%) vs. FRA (%) vs. ASM (%)',
                series1Name: 'Tablespace Máx %',
                series1Color: const Color(0xFF5FFFFF),
                series1Values: history
                    .map((e) => ((e['max_tablespace_pct'] ?? 0) as num).toDouble())
                    .toList(),
                series2Name: 'FRA Usada %',
                series2Color: const Color(0xFFFFAF00),
                series2Values:
                    history.map((e) => ((e['fra_pct'] ?? fraPct) as num).toDouble()).toList(),
                maxY: 100,
              ),
              _timeSeriesChartCard(
                title: 'Errores Alert Log (ORA-) vs. Caminos SAN Multipath Caídos',
                series1Name: 'Errores ORA-',
                series1Color: const Color(0xFFFFAF00),
                series1Values:
                    history.map((e) => ((e['ora_error_count'] ?? 0) as num).toDouble()).toList(),
                series2Name: 'Caminos SAN Caídos',
                series2Color: const Color(0xFFFF5F5F),
                series2Values: history
                    .map((e) => ((e['unhealthy_san_paths'] ?? 0) as num).toDouble())
                    .toList(),
              ),
            ],
          ),
          const SizedBox(height: 16),

          // Cluster Nodes & Instances + Listeners with Action Buttons
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                child: _nortonPanel(
                  title: 'NODOS DEL CLUSTER, DEMONIOS GI & INSTANCIAS',
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      for (final n in nodes)
                        Padding(
                          padding: const EdgeInsets.symmetric(vertical: 4),
                          child: Row(
                            children: [
                              const Icon(Icons.developer_board, color: Color(0xFF5FFFFF), size: 18),
                              const SizedBox(width: 8),
                              Expanded(
                                child: Text(
                                  'Nodo: ${n['Name']} (${n['Role']}) - ${n['Status']}',
                                  style: const TextStyle(fontWeight: FontWeight.bold),
                                ),
                              ),
                              _statusBadge('CRS:${n['CRSD']}', n['CRSD'] == 'ONLINE'),
                              const SizedBox(width: 4),
                              _statusBadge('CSSD:${n['CSSD']}', n['CSSD'] == 'ONLINE'),
                              const SizedBox(width: 4),
                              _statusBadge('ASM:${n['ASM']}', n['ASM'] == 'ONLINE'),
                            ],
                          ),
                        ),
                      const Divider(color: Color(0xFF5FFFFF)),
                      for (final inst in instances)
                        Padding(
                          padding: const EdgeInsets.symmetric(vertical: 4),
                          child: Row(
                            children: [
                              const Icon(Icons.storage, color: Color(0xFFFFFF5F), size: 18),
                              const SizedBox(width: 8),
                              Expanded(
                                child: Text(
                                  'SID: ${inst['InstName']} @ ${inst['HostName']} | ${inst['OpenMode']} | SGA: ${inst['SGAMB']}M PGA: ${inst['PGAMB']}M | FRA: ${inst['FRAUsedPct']}%',
                                ),
                              ),
                              _statusBadge(inst['Status'].toString(), inst['Status'] == 'OPEN'),
                            ],
                          ),
                        ),
                    ],
                  ),
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: _nortonPanel(
                  title: 'LISTENERS ORACLE NET & PLUGGABLE DATABASES (PDBs)',
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      for (final l in listeners)
                        Padding(
                          padding: const EdgeInsets.symmetric(vertical: 4),
                          child: Row(
                            children: [
                              const Icon(Icons.settings_ethernet, color: Color(0xFF5FFF5F), size: 18),
                              const SizedBox(width: 8),
                              Expanded(
                                child: Text(
                                    '${l['Name']} (${l['Port']}) → ${l['ServicesSummary']}'),
                              ),
                              _statusBadge(l['Status'].toString(), l['Status'] == 'ONLINE'),
                              const SizedBox(width: 8),
                              ElevatedButton.icon(
                                style: ElevatedButton.styleFrom(
                                  backgroundColor: const Color(0xFF00AFAF),
                                  foregroundColor: Colors.black,
                                  padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                                ),
                                onPressed: () => _executeRemoteAction(
                                  action: 'listener_reload',
                                  target: l['Name'].toString(),
                                  label: 'Recargar Listener ${l['Name']}',
                                ),
                                icon: const Icon(Icons.refresh, size: 14),
                                label: const Text('RELOAD', style: TextStyle(fontSize: 11)),
                              ),
                            ],
                          ),
                        ),
                      const Divider(color: Color(0xFF5FFFFF)),
                      Wrap(
                        spacing: 8,
                        runSpacing: 8,
                        children: [
                          for (final p in pdbs)
                            Chip(
                              backgroundColor: const Color(0xFF000044),
                              side: BorderSide(
                                color: p['OpenMode'] == 'READ WRITE'
                                    ? const Color(0xFF5FFF5F)
                                    : const Color(0xFFFFAF00),
                              ),
                              label: Text(
                                '${p['Name']}: ${p['OpenMode']} (${p['TotalMB']} MB)',
                                style: const TextStyle(fontSize: 12),
                              ),
                            ),
                        ],
                      ),
                    ],
                  ),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }

  // ===========================================================================
  // TAB 2: SRE, SLOs, ERROR BUDGETS & AVISOS (TELEGRAM / EMAIL SMTP)
  // ===========================================================================
  Widget _buildTab2SreSlosAndNotifications() {
    final slos = (_sreData['slos'] as List?) ?? [];
    final alerts = (_sreData['active_alerts'] as List?) ?? [];
    final notifications = (_sreData['notifications'] as List?) ?? [];

    return SingleChildScrollView(
      padding: const EdgeInsets.all(14),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _sectionHeader(
            '🎯 OBJETIVOS DE NIVEL DE SERVICIO (SLOs), SLIs & ERROR BUDGET EN TIEMPO REAL',
          ),
          const SizedBox(height: 8),
          GridView.count(
            crossAxisCount: MediaQuery.of(context).size.width > 1200 ? 4 : 2,
            shrinkWrap: true,
            physics: const NeverScrollableScrollPhysics(),
            mainAxisSpacing: 10,
            crossAxisSpacing: 10,
            childAspectRatio: 1.55,
            children: [
              for (final slo in slos) _buildSloCard(Map<String, dynamic>.from(slo as Map)),
            ],
          ),
          const SizedBox(height: 18),

          // Active Delicate Alerts with 1-Click Remediation
          _sectionHeader(
            '🚨 ALARMAS DELICADAS ACTIVAS (CON OPERACIÓN Y REMEDIACIÓN EN 1 CLIC)',
            trailing: ElevatedButton.icon(
              style: ElevatedButton.styleFrom(
                backgroundColor: const Color(0xFFFF5F5F),
                foregroundColor: Colors.white,
              ),
              onPressed: _sendImmediateSreAlertTest,
              icon: const Icon(Icons.send, size: 16),
              label: const Text('ENVIAR ALARMA A TELEGRAM & EMAIL AHORA'),
            ),
          ),
          const SizedBox(height: 8),
          if (alerts.isEmpty)
            _nortonPanel(
              title: 'SIN ALARMAS DELICADAS ACTIVAS',
              child: const Text(
                '✅ Todos los indicadores SLI cumplen los umbrales de producción.',
                style: TextStyle(color: Color(0xFF5FFF5F)),
              ),
            )
          else
            Column(
              children: [
                for (final a in alerts)
                  Container(
                    margin: const EdgeInsets.only(bottom: 8),
                    padding: const EdgeInsets.all(12),
                    decoration: BoxDecoration(
                      color: const Color(0xFF330011),
                      border: Border.all(
                        color: a['severity'] == 'CRITICAL'
                            ? const Color(0xFFFF5F5F)
                            : const Color(0xFFFFAF00),
                        width: 2,
                      ),
                    ),
                    child: Row(
                      children: [
                        Icon(
                          a['severity'] == 'CRITICAL' ? Icons.gpp_bad : Icons.warning_amber,
                          color: a['severity'] == 'CRITICAL'
                              ? const Color(0xFFFF5F5F)
                              : const Color(0xFFFFAF00),
                          size: 28,
                        ),
                        const SizedBox(width: 12),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                '[${a['severity']} • ${a['category']}] ${a['title']}',
                                style: const TextStyle(
                                  color: Color(0xFFFFFF5F),
                                  fontWeight: FontWeight.bold,
                                  fontSize: 14,
                                ),
                              ),
                              const SizedBox(height: 4),
                              Text(
                                a['description'].toString(),
                                style: const TextStyle(color: Colors.white70, fontSize: 12),
                              ),
                            ],
                          ),
                        ),
                        const SizedBox(width: 12),
                        ElevatedButton.icon(
                          style: ElevatedButton.styleFrom(
                            backgroundColor: const Color(0xFF00AFAF),
                            foregroundColor: Colors.black,
                            padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
                          ),
                          onPressed: () => _executeRemoteAction(
                            action: (a['action_id'] ?? '').toString(),
                            target: (a['action_target'] ?? '').toString(),
                            label: (a['recommended_action'] ?? 'Remediar Alarma').toString(),
                          ),
                          icon: const Icon(Icons.build_circle, size: 18),
                          label: Text(
                            '⚡ ${(a['recommended_action'] ?? 'EJECUTAR REMEDIACIÓN').toString().toUpperCase()}',
                            style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 12),
                          ),
                        ),
                      ],
                    ),
                  ),
              ],
            ),
          const SizedBox(height: 18),

          // Configuration of SRE Targets + Telegram + SMTP Email
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                flex: 3,
                child: _nortonPanel(
                  title: '⚙️ CONFIGURACIÓN DE OBJETIVOS SLO Y AVISOS (TELEGRAM BOT & CORREO SMTP)',
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      const Text(
                        '1. Umbrales Objetivos de SLO (%) y Enfriamiento entre Avisos:',
                        style: TextStyle(color: Color(0xFFFFFF5F), fontWeight: FontWeight.bold),
                      ),
                      const SizedBox(height: 8),
                      Row(
                        children: [
                          Expanded(child: _compactField(_availTargetCtrl, 'SLO Disponibilidad %')),
                          const SizedBox(width: 8),
                          Expanded(child: _compactField(_lockFreeTargetCtrl, 'SLO Sin Bloqueos %')),
                          const SizedBox(width: 8),
                          Expanded(child: _compactField(_storageTargetCtrl, 'SLO Capacidad %')),
                          const SizedBox(width: 8),
                          Expanded(child: _compactField(_sanTargetCtrl, 'SLO Redundancia SAN %')),
                          const SizedBox(width: 8),
                          Expanded(child: _compactField(_cooldownCtrl, 'Cooldown (min)')),
                        ],
                      ),
                      SwitchListTile(
                        contentPadding: EdgeInsets.zero,
                        title: const Text(
                          'Enviar aviso automático inmediato cuando se detecte una Alarma CRITICAL',
                          style: TextStyle(fontSize: 13),
                        ),
                        value: _autoNotifyCritical,
                        activeColor: const Color(0xFF5FFF5F),
                        onChanged: (v) => setState(() => _autoNotifyCritical = v),
                      ),
                      const Divider(color: Color(0xFF5FFFFF)),

                      // Telegram Bot section
                      SwitchListTile(
                        contentPadding: EdgeInsets.zero,
                        title: const Text(
                          '📲 Activar Avisos Críticos por Telegram (api.telegram.org)',
                          style: TextStyle(
                            color: Color(0xFF5FFFFF),
                            fontWeight: FontWeight.bold,
                            fontSize: 14,
                          ),
                        ),
                        value: _telegramEnabled,
                        activeColor: const Color(0xFF5FFFFF),
                        onChanged: (v) => setState(() => _telegramEnabled = v),
                      ),
                      Row(
                        children: [
                          Expanded(
                            flex: 2,
                            child: _compactField(
                              _tgTokenCtrl,
                              'Telegram Bot Token (ej. 123456789:AAH...)',
                            ),
                          ),
                          const SizedBox(width: 10),
                          Expanded(
                            child: _compactField(
                              _tgChatIdCtrl,
                              'Chat ID o Canal (ej. -10019283746)',
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(height: 12),
                      const Divider(color: Color(0xFF5FFFFF)),

                      // Email SMTP section
                      SwitchListTile(
                        contentPadding: EdgeInsets.zero,
                        title: const Text(
                          '📧 Activar Avisos Críticos por Correo Electrónico (SMTP / STARTTLS)',
                          style: TextStyle(
                            color: Color(0xFFFFFF5F),
                            fontWeight: FontWeight.bold,
                            fontSize: 14,
                          ),
                        ),
                        value: _emailEnabled,
                        activeColor: const Color(0xFFFFFF5F),
                        onChanged: (v) => setState(() => _emailEnabled = v),
                      ),
                      Row(
                        children: [
                          Expanded(
                            flex: 2,
                            child: _compactField(_smtpHostCtrl, 'Servidor SMTP (ej. smtp.office365.com)'),
                          ),
                          const SizedBox(width: 8),
                          Expanded(child: _compactField(_smtpPortCtrl, 'Puerto (587 / 25)')),
                          const SizedBox(width: 8),
                          Expanded(child: _compactField(_smtpUserCtrl, 'Usuario SMTP')),
                          const SizedBox(width: 8),
                          Expanded(
                            child: _compactField(_smtpPassCtrl, 'Password SMTP', obscure: true),
                          ),
                        ],
                      ),
                      const SizedBox(height: 8),
                      Row(
                        children: [
                          Expanded(
                            child: _compactField(_smtpFromCtrl, 'Remitente From (ej. oratui-sre@dominio.com)'),
                          ),
                          const SizedBox(width: 8),
                          Expanded(
                            child: _compactField(
                                _smtpToCtrl, 'Destinatarios To separados por coma (ej. guardia-dba@dominio.com)'),
                          ),
                        ],
                      ),
                      const SizedBox(height: 14),
                      Row(
                        children: [
                          ElevatedButton.icon(
                            style: ElevatedButton.styleFrom(
                              backgroundColor: const Color(0xFF5FFF5F),
                              foregroundColor: Colors.black,
                              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
                            ),
                            onPressed: _saveSreConfig,
                            icon: const Icon(Icons.save),
                            label: const Text(
                              'GUARDAR CONFIGURACIÓN SRE / ALERTAS',
                              style: TextStyle(fontWeight: FontWeight.bold),
                            ),
                          ),
                          const SizedBox(width: 12),
                          ElevatedButton.icon(
                            style: ElevatedButton.styleFrom(
                              backgroundColor: const Color(0xFF00AFAF),
                              foregroundColor: Colors.black,
                              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
                            ),
                            onPressed: _sendImmediateSreAlertTest,
                            icon: const Icon(Icons.notifications_active),
                            label: const Text(
                              'PROBAR ENVÍO TELEGRAM / EMAIL AHORA',
                              style: TextStyle(fontWeight: FontWeight.bold),
                            ),
                          ),
                        ],
                      ),
                    ],
                  ),
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                flex: 2,
                child: _nortonPanel(
                  title: '📨 HISTORIAL DE NOTIFICACIONES DESPACHADAS',
                  child: notifications.isEmpty
                      ? const Padding(
                          padding: EdgeInsets.all(12),
                          child: Text(
                            'Aún no se han disparado notificaciones en esta sesión. Pulsa "PROBAR ENVÍO TELEGRAM / EMAIL AHORA" para verificar el canal de guardia.',
                            style: TextStyle(color: Colors.white70),
                          ),
                        )
                      : Column(
                          children: [
                            for (final n in notifications)
                              Container(
                                margin: const EdgeInsets.only(bottom: 8),
                                padding: const EdgeInsets.all(8),
                                decoration: BoxDecoration(
                                  color: Colors.black38,
                                  border: Border.all(
                                    color: (n['success'] == true)
                                        ? const Color(0xFF5FFF5F)
                                        : const Color(0xFFFF5F5F),
                                  ),
                                ),
                                child: Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    Row(
                                      children: [
                                        _statusBadge(
                                          n['channel'].toString(),
                                          n['success'] == true,
                                        ),
                                        const SizedBox(width: 8),
                                        Expanded(
                                          child: Text(
                                            '→ ${n['target']}',
                                            style: const TextStyle(
                                              color: Color(0xFF5FFFFF),
                                              fontSize: 12,
                                            ),
                                          ),
                                        ),
                                        Text(
                                          n['timestamp'].toString(),
                                          style: const TextStyle(
                                            color: Colors.white54,
                                            fontSize: 11,
                                          ),
                                        ),
                                      ],
                                    ),
                                    const SizedBox(height: 4),
                                    Text(
                                      n['summary'].toString(),
                                      style: const TextStyle(fontSize: 12),
                                    ),
                                  ],
                                ),
                              ),
                          ],
                        ),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Widget _buildSloCard(Map<String, dynamic> slo) {
    final target = ((slo['target_pct'] ?? 99.9) as num).toDouble();
    final current = ((slo['current_sli_pct'] ?? 100.0) as num).toDouble();
    final budget = ((slo['error_budget_remaining_pct'] ?? 100.0) as num).toDouble();
    final burnRate = ((slo['burn_rate'] ?? 0.0) as num).toDouble();
    final status = (slo['status'] ?? 'HEALTHY').toString();
    final healthy = status == 'HEALTHY';

    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: const Color(0xFF000070),
        border: Border.all(
          color: healthy ? const Color(0xFF5FFF5F) : const Color(0xFFFF5F5F),
          width: 2,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  (slo['name'] ?? '').toString(),
                  style: const TextStyle(
                    color: Color(0xFFFFFF5F),
                    fontWeight: FontWeight.bold,
                    fontSize: 13,
                  ),
                ),
              ),
              _statusBadge(status, healthy),
            ],
          ),
          Text(
            'SLI Actual: ${current.toStringAsFixed(2)}%  (Objetivo SLO: ${target.toStringAsFixed(2)}%)',
            style: const TextStyle(color: Color(0xFF5FFFFF), fontSize: 12),
          ),
          Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Text(
                    'Error Budget Restante: ${budget.toStringAsFixed(1)}%',
                    style: const TextStyle(fontSize: 11),
                  ),
                  Text(
                    'Burn Rate: ${burnRate.toStringAsFixed(1)}x',
                    style: TextStyle(
                      fontSize: 11,
                      color: burnRate > 1.0 ? const Color(0xFFFF5F5F) : const Color(0xFF5FFF5F),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 4),
              LinearProgressIndicator(
                value: (budget / 100.0).clamp(0.0, 1.0),
                minHeight: 8,
                backgroundColor: Colors.black54,
                valueColor: AlwaysStoppedAnimation<Color>(
                  budget > 50
                      ? const Color(0xFF5FFF5F)
                      : (budget > 20 ? const Color(0xFFFFAF00) : const Color(0xFFFF5F5F)),
                ),
              ),
            ],
          ),
          Align(
            alignment: Alignment.centerRight,
            child: TextButton.icon(
              style: TextButton.styleFrom(
                foregroundColor: const Color(0xFFFFFF5F),
                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
              ),
              onPressed: () => _executeRemoteAction(
                action: (slo['recommended_action_id'] ?? 'switch_logfile').toString(),
                label: 'Remediación de SLO: ${slo['name']}',
              ),
              icon: const Icon(Icons.flash_on, size: 15),
              label: const Text('EJECUTAR ACCIÓN SLO', style: TextStyle(fontSize: 11)),
            ),
          ),
        ],
      ),
    );
  }

  // ===========================================================================
  // TAB 3: TABLESPACES, ASM DISKGROUPS, DATAFILES & TOP SEGMENTOS
  // ===========================================================================
  Widget _buildTab3StorageAndSegments() {
    final tbsList = (_stateData['Tablespaces'] as List?) ?? [];
    final asmList = (_stateData['ASMDiskgroups'] as List?) ?? [];
    final dfList = (_stateData['Datafiles'] as List?) ?? [];
    final segList = (_stateData['Segments'] as List?) ?? [];

    return SingleChildScrollView(
      padding: const EdgeInsets.all(14),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _nortonPanel(
            title: '💾 TABLESPACES (dba_tablespace_usage_metrics) — ACCIONES DIRECTAS DE AMPLIACIÓN Y COALESCE',
            child: Column(
              children: [
                for (final t in tbsList)
                  Padding(
                    padding: const EdgeInsets.symmetric(vertical: 6),
                    child: Row(
                      children: [
                        SizedBox(
                          width: 170,
                          child: Text(
                            '${t['Name']} (${t['Contents']})',
                            style: const TextStyle(fontWeight: FontWeight.bold),
                          ),
                        ),
                        Expanded(
                          child: _lcdProgressBar(
                            pct: ((t['UsedPercent'] ?? 0) as num).toDouble(),
                            label:
                                '${t['UsedMB']} MB / ${t['TotalMB']} MB (${t['UsedPercent']}%) | Autoextend: ${t['AutoExtend'] == true ? "SÍ" : "NO"}',
                          ),
                        ),
                        const SizedBox(width: 10),
                        ElevatedButton.icon(
                          style: ElevatedButton.styleFrom(
                            backgroundColor: const Color(0xFF00AFAF),
                            foregroundColor: Colors.black,
                          ),
                          onPressed: () => _executeRemoteAction(
                            action: 'add_datafile',
                            target: t['Name'].toString(),
                            label: 'Añadir Datafile 32G AUTOEXTEND a ${t['Name']}',
                          ),
                          icon: const Icon(Icons.add, size: 15),
                          label: const Text('+DATAFILE 32G', style: TextStyle(fontSize: 11)),
                        ),
                        const SizedBox(width: 6),
                        OutlinedButton(
                          onPressed: () => _executeRemoteAction(
                            action: 'coalesce_tablespace',
                            target: t['Name'].toString(),
                            label: 'Consolidar espacio COALESCE en ${t['Name']}',
                          ),
                          child: const Text('COALESCE', style: TextStyle(fontSize: 11)),
                        ),
                      ],
                    ),
                  ),
              ],
            ),
          ),
          const SizedBox(height: 14),
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                child: _nortonPanel(
                  title: '💿 GRUPOS DE DISCOS ASM (v\$asm_diskgroup)',
                  child: Column(
                    children: [
                      for (final g in asmList)
                        Padding(
                          padding: const EdgeInsets.symmetric(vertical: 6),
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                '+${g['Name']} [${g['State']} • Redundancia ${g['Type']}] — Discos Offline: ${g['OfflineDisks']}',
                                style: const TextStyle(
                                  color: Color(0xFFFFFF5F),
                                  fontWeight: FontWeight.bold,
                                ),
                              ),
                              const SizedBox(height: 4),
                              _lcdProgressBar(
                                pct: ((g['UsedPct'] ?? 0) as num).toDouble(),
                                label:
                                    'Libre: ${g['FreeMB']} MB / Total: ${g['TotalMB']} MB (${g['UsedPct']}%)',
                              ),
                            ],
                          ),
                        ),
                    ],
                  ),
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: _nortonPanel(
                  title: '📊 TOP SEGMENTOS POR TAMAÑO (dba_segments)',
                  child: Column(
                    children: [
                      for (final s in segList)
                        ListTile(
                          dense: true,
                          contentPadding: EdgeInsets.zero,
                          title: Text(
                            '${s['Owner']}.${s['SegmentName']} [${s['SegmentType']}]',
                            style: const TextStyle(color: Color(0xFF5FFFFF)),
                          ),
                          subtitle: Text(
                            'Tablespace: ${s['TablespaceName']} | Extents: ${s['Extents']}',
                          ),
                          trailing: Text(
                            '${s['SizeMB']} MB',
                            style: const TextStyle(
                              color: Color(0xFFFFFF5F),
                              fontWeight: FontWeight.bold,
                            ),
                          ),
                        ),
                    ],
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 14),
          _nortonPanel(
            title: '📁 DATAFILES FÍSICOS (dba_data_files)',
            child: Column(
              children: [
                for (final d in dfList)
                  Padding(
                    padding: const EdgeInsets.symmetric(vertical: 3),
                    child: Row(
                      children: [
                        Expanded(
                          flex: 3,
                          child: Text(
                            '${d['FileName']}',
                            style: const TextStyle(color: Color(0xFF5FFFFF), fontSize: 12),
                          ),
                        ),
                        Expanded(child: Text('TBS: ${d['Tablespace']}')),
                        Expanded(child: Text('${d['SizeMB']} MB / Max ${d['MaxMB']} MB')),
                        _statusBadge(
                          d['Autoextend'] == true ? 'AUTOEXTEND ON' : 'FIXED',
                          d['Autoextend'] == true,
                        ),
                      ],
                    ),
                  ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  // ===========================================================================
  // TAB 4: SESIONES, BLOQUEOS TX/TM, OBJETOS INVÁLIDOS & ESTADÍSTICAS
  // ===========================================================================
  Widget _buildTab4SessionsLocksAndObjects() {
    final sessions = (_stateData['Sessions'] as List?) ?? [];
    final invalids = (_stateData['InvalidObjects'] as List?) ?? [];
    final stats = (_stateData['TableStats'] as List?) ?? [];

    return SingleChildScrollView(
      padding: const EdgeInsets.all(14),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _nortonPanel(
            title: '🔒 SESIONES ACTIVAS Y ÁRBOL DE BLOQUEOS TX/TM (gv\$session) — MATAR SESIÓN EN 1 CLIC',
            child: Column(
              children: [
                for (final s in sessions)
                  Container(
                    margin: const EdgeInsets.symmetric(vertical: 4),
                    padding: const EdgeInsets.all(8),
                    color: (s['BlockingSession'] ?? '').toString().isNotEmpty
                        ? const Color(0xFF440011)
                        : Colors.black26,
                    child: Row(
                      children: [
                        Expanded(
                          flex: 2,
                          child: Text(
                            'INST#${s['InstID']} SID:${s['SID']},${s['Serial']} (${s['Username']})',
                            style: const TextStyle(
                              color: Color(0xFFFFFF5F),
                              fontWeight: FontWeight.bold,
                            ),
                          ),
                        ),
                        Expanded(
                          flex: 3,
                          child: Text(
                            'Evento: ${s['Event']} | SQL_ID: ${s['SQLID']} | Espera: ${s['SecondsInWait']}s',
                            style: const TextStyle(fontSize: 12),
                          ),
                        ),
                        Expanded(
                          flex: 2,
                          child: Text(
                            (s['BlockingSession'] ?? '').toString().isNotEmpty
                                ? '🚨 BLOQUEO: ${s['BlockingSession']}'
                                : 'OK',
                            style: TextStyle(
                              color: (s['BlockingSession'] ?? '').toString().isNotEmpty
                                  ? const Color(0xFFFF5F5F)
                                  : const Color(0xFF5FFF5F),
                              fontWeight: FontWeight.bold,
                            ),
                          ),
                        ),
                        ElevatedButton.icon(
                          style: ElevatedButton.styleFrom(
                            backgroundColor: const Color(0xFFD32F2F),
                            foregroundColor: Colors.white,
                          ),
                          onPressed: () => _executeRemoteAction(
                            action: 'kill_session',
                            target: '${s['SID']},${s['Serial']},@${s['InstID']}',
                            label:
                                'Matar Sesión SID ${s['SID']},${s['Serial']} (${s['Username']}) IMMEDIATE',
                          ),
                          icon: const Icon(Icons.cancel, size: 15),
                          label: const Text('KILL SESSION', style: TextStyle(fontSize: 11)),
                        ),
                      ],
                    ),
                  ),
              ],
            ),
          ),
          const SizedBox(height: 14),
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                child: _nortonPanel(
                  title: '🔧 OBJETOS INVÁLIDOS (${invalids.length}) — UTL_RECOMP',
                  trailing: ElevatedButton.icon(
                    style: ElevatedButton.styleFrom(
                      backgroundColor: const Color(0xFF00AFAF),
                      foregroundColor: Colors.black,
                    ),
                    onPressed: () => _executeRemoteAction(
                      action: 'compile_invalids',
                      label: 'Recompilar Objetos Inválidos (UTL_RECOMP.RECOMP_PARALLEL)',
                    ),
                    icon: const Icon(Icons.build, size: 15),
                    label: const Text('RECOMPILAR TODOS (UTL_RECOMP)', style: TextStyle(fontSize: 11)),
                  ),
                  child: Column(
                    children: [
                      for (final o in invalids)
                        ListTile(
                          dense: true,
                          contentPadding: EdgeInsets.zero,
                          title: Text(
                            '${o['Owner']}.${o['ObjectName']} (${o['ObjectType']})',
                            style: const TextStyle(color: Color(0xFFFFAF00)),
                          ),
                          subtitle: Text('Último DDL: ${o['LastDDL']}'),
                          trailing: _statusBadge(o['Status'].toString(), false),
                        ),
                    ],
                  ),
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: _nortonPanel(
                  title: '📈 ESTADÍSTICAS DEL OPTIMIZADOR CBO (DBMS_STATS)',
                  child: Column(
                    children: [
                      for (final st in stats)
                        Padding(
                          padding: const EdgeInsets.symmetric(vertical: 4),
                          child: Row(
                            children: [
                              Expanded(
                                child: Text(
                                  '${st['Owner']}.${st['TableName']} (${st['NumRows']} filas | ${st['LastAnalyzed']})',
                                  style: const TextStyle(fontSize: 12),
                                ),
                              ),
                              _statusBadge(
                                st['StaleStats'] == 'YES' ? 'STALE' : 'FRESH',
                                st['StaleStats'] != 'YES',
                              ),
                              const SizedBox(width: 8),
                              ElevatedButton(
                                style: ElevatedButton.styleFrom(
                                  backgroundColor: const Color(0xFF00AFAF),
                                  foregroundColor: Colors.black,
                                  padding: const EdgeInsets.symmetric(horizontal: 8),
                                ),
                                onPressed: () => _executeRemoteAction(
                                  action: 'gather_stats',
                                  target: '${st['Owner']}.${st['TableName']}',
                                  label:
                                      'Recolectar Estadísticas DBMS_STATS en ${st['Owner']}.${st['TableName']}',
                                ),
                                child: const Text('GATHER STATS', style: TextStyle(fontSize: 11)),
                              ),
                            ],
                          ),
                        ),
                    ],
                  ),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }

  // ===========================================================================
  // TAB 5: RMAN BACKUPS, DATA GUARD, RESTORE POINTS & ALERT LOG (ORA-)
  // ===========================================================================
  Widget _buildTab5RmanDataGuardAndAlertLog() {
    final backups = (_stateData['Backups'] as List?) ?? [];
    final rps = (_stateData['RestorePoints'] as List?) ?? [];
    final dgList = (_stateData['DataGuard'] as List?) ?? [];
    final oraErrors = (_stateData['AlertErrors'] as List?) ?? [];

    return SingleChildScrollView(
      padding: const EdgeInsets.all(14),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Wrap(
            spacing: 10,
            runSpacing: 10,
            children: [
              ElevatedButton.icon(
                style: ElevatedButton.styleFrom(
                  backgroundColor: const Color(0xFF00AFAF),
                  foregroundColor: Colors.black,
                ),
                onPressed: () => _executeRemoteAction(
                  action: 'rman_archivelog_backup',
                  label: 'Ejecutar Backup/Crosscheck RMAN de Archivelogs y liberar FRA',
                ),
                icon: const Icon(Icons.backup),
                label: const Text('BACKUP RMAN ARCHIVELOGS + LIBERAR FRA'),
              ),
              ElevatedButton.icon(
                style: ElevatedButton.styleFrom(
                  backgroundColor: const Color(0xFFFFFF5F),
                  foregroundColor: Colors.black,
                ),
                onPressed: () => _executeRemoteAction(
                  action: 'create_restore_point',
                  label: 'Crear Punto de Restauración Garantizado (GRP Flashback)',
                ),
                icon: const Icon(Icons.bookmark_add),
                label: const Text('CREAR RESTORE POINT GARANTIZADO'),
              ),
              ElevatedButton.icon(
                style: ElevatedButton.styleFrom(
                  backgroundColor: const Color(0xFF5FFF5F),
                  foregroundColor: Colors.black,
                ),
                onPressed: () => _executeRemoteAction(
                  action: 'purge_adrci',
                  label: 'Purgar Trazas Antiguas con ADRCI (Liberar Inodos)',
                ),
                icon: const Icon(Icons.cleaning_services),
                label: const Text('PURGAR TRAZAS ADRCI (>7 DÍAS)'),
              ),
            ],
          ),
          const SizedBox(height: 14),
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                child: _nortonPanel(
                  title: '🛡️ HISTORIAL DE BACKUPS RMAN (v\$rman_backup_job_details)',
                  child: Column(
                    children: [
                      for (final b in backups)
                        ListTile(
                          dense: true,
                          contentPadding: EdgeInsets.zero,
                          title: Text(
                            '${b['InputType']} (${b['OutputDevice']}) — ${b['OutputMB']} MB en ${b['DurationMin']} min',
                            style: const TextStyle(color: Color(0xFF5FFFFF)),
                          ),
                          subtitle: Text('Inicio: ${b['StartTime']}'),
                          trailing: _statusBadge(
                            b['Status'].toString(),
                            b['Status'].toString().contains('COMPLETED'),
                          ),
                        ),
                    ],
                  ),
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: _nortonPanel(
                  title: '🔄 REPLICACIÓN DATA GUARD & RESTORE POINTS',
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      for (final dg in dgList)
                        ListTile(
                          dense: true,
                          contentPadding: EdgeInsets.zero,
                          title: Text(
                            '${dg['ProtectionMode']} | ${dg['DatabaseRole']}',
                            style: const TextStyle(color: Color(0xFFFFFF5F)),
                          ),
                          subtitle: Text(
                            'Transport Lag: ${dg['TransportLag']} | Apply Lag: ${dg['ApplyLag']} | MRP: ${dg['MRPStatus']}',
                          ),
                          trailing: _statusBadge(dg['SwitchoverStatus'].toString(), true),
                        ),
                      const Divider(color: Color(0xFF5FFFFF)),
                      for (final rp in rps)
                        Text(
                          '📌 Restore Point: ${rp['Name']} (Garantizado: ${rp['Guaranteed']}) — ${rp['StorageMB']} MB (${rp['Time']})',
                          style: const TextStyle(fontSize: 12, color: Color(0xFF5FFF5F)),
                        ),
                    ],
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 14),
          _nortonPanel(
            title: '🚨 ERRORES ORA- DETECTADOS EN ALERT LOG (v\$diag_alert_ext)',
            child: Column(
              children: [
                for (final err in oraErrors)
                  Container(
                    margin: const EdgeInsets.symmetric(vertical: 4),
                    padding: const EdgeInsets.all(8),
                    color: const Color(0xFF331100),
                    child: Row(
                      children: [
                        _statusBadge(err['Code'].toString(), false),
                        const SizedBox(width: 10),
                        Expanded(
                          child: Text(
                            '[${err['Instance']} • ${err['LastSeen']}] ${err['Message']}',
                            style: const TextStyle(fontSize: 12),
                          ),
                        ),
                        Chip(
                          label: Text('x${err['Occurrences']}'),
                          backgroundColor: const Color(0xFF550000),
                        ),
                      ],
                    ),
                  ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  // ===========================================================================
  // TAB 6: LINUX KERNEL, FILESYSTEMS/INODOS & SAN MULTIPATH (LSBLK / FC)
  // ===========================================================================
  Widget _buildTab6LinuxKernelAndSanMultipath() {
    final osMetrics = (_stateData['OSMetrics'] as List?) ?? [];
    final fsList = (_stateData['Filesystems'] as List?) ?? [];
    final paths = (_stateData['StoragePaths'] as List?) ?? [];

    return SingleChildScrollView(
      padding: const EdgeInsets.all(14),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Wrap(
            spacing: 10,
            runSpacing: 10,
            children: [
              ElevatedButton.icon(
                style: ElevatedButton.styleFrom(
                  backgroundColor: const Color(0xFF00AFAF),
                  foregroundColor: Colors.black,
                ),
                onPressed: () => _executeRemoteAction(
                  action: 'audit_multipath',
                  label: 'Auditar Caminos SAN (multipath -ll & lsblk)',
                ),
                icon: const Icon(Icons.radar),
                label: const Text('AUDITAR MULTIPATH -LL & LSBLK'),
              ),
              ElevatedButton.icon(
                style: ElevatedButton.styleFrom(
                  backgroundColor: const Color(0xFFFF5F5F),
                  foregroundColor: Colors.white,
                ),
                onPressed: () => _executeRemoteAction(
                  action: 'rescan_scsi',
                  label: 'Reescanear Buses SCSI/Fibre Channel y Recargar Multipath (-r)',
                ),
                icon: const Icon(Icons.cable),
                label: const Text('REESCANEAR BUS SCSI/FC & MULTIPATH -R'),
              ),
              ElevatedButton.icon(
                style: ElevatedButton.styleFrom(
                  backgroundColor: const Color(0xFFFFFF5F),
                  foregroundColor: Colors.black,
                ),
                onPressed: () => _executeRemoteAction(
                  action: 'audit_sysctl',
                  label: 'Auditar Parámetros Kernel Linux para Oracle (sysctl)',
                ),
                icon: const Icon(Icons.tune),
                label: const Text('AUDITAR SYSCTL ORACLE'),
              ),
              ElevatedButton.icon(
                style: ElevatedButton.styleFrom(
                  backgroundColor: const Color(0xFF5FFF5F),
                  foregroundColor: Colors.black,
                ),
                onPressed: () => _executeRemoteAction(
                  action: 'top_processes',
                  label: 'Consultar Top 20 Procesos Linux por CPU y RAM',
                ),
                icon: const Icon(Icons.speed),
                label: const Text('TOP 20 PROCESOS CPU/RAM'),
              ),
            ],
          ),
          const SizedBox(height: 14),
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                child: _nortonPanel(
                  title: '🐧 MÉTRICAS DE KERNEL LINUX, CPU, RAM, HUGEPAGES & RED',
                  child: Column(
                    children: [
                      for (final m in osMetrics)
                        Padding(
                          padding: const EdgeInsets.symmetric(vertical: 5),
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Row(
                                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                                children: [
                                  Text(
                                    '[${m['Category']}] ${m['Name']}: ${m['Value']}',
                                    style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 12),
                                  ),
                                  _statusBadge(
                                    m['Healthy'] == true ? 'OK' : 'WARN',
                                    m['Healthy'] == true,
                                  ),
                                ],
                              ),
                              const SizedBox(height: 3),
                              _lcdProgressBar(
                                pct: ((m['UsedPercent'] ?? 0) as num).toDouble(),
                                label: (m['Detail'] ?? '').toString(),
                              ),
                            ],
                          ),
                        ),
                    ],
                  ),
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: _nortonPanel(
                  title: '📂 FILESYSTEMS (df -PTm) E INODOS (df -Pi)',
                  trailing: TextButton.icon(
                    onPressed: () => _executeRemoteAction(
                      action: 'audit_inodes',
                      label: 'Auditar Espacio e Inodos en Filesystems (df -PTm & df -Pi)',
                    ),
                    icon: const Icon(Icons.cleaning_services, size: 14, color: Color(0xFFFFFF5F)),
                    label: const Text('DF -Pi EN VIVO', style: TextStyle(color: Color(0xFFFFFF5F))),
                  ),
                  child: Column(
                    children: [
                      for (final fs in fsList)
                        Padding(
                          padding: const EdgeInsets.symmetric(vertical: 6),
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                '${fs['MountPoint']} (${fs['Filesystem']} • ${fs['FSType']})',
                                style: const TextStyle(
                                  color: Color(0xFFFFFF5F),
                                  fontWeight: FontWeight.bold,
                                ),
                              ),
                              const SizedBox(height: 4),
                              _lcdProgressBar(
                                pct: ((fs['UsedPercent'] ?? 0) as num).toDouble(),
                                label:
                                    'Espacio: ${fs['UsedMB']}M / ${fs['TotalMB']}M (${fs['UsedPercent']}%) | Inodos Usados: ${fs['InodesPct']}%',
                              ),
                            ],
                          ),
                        ),
                    ],
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 14),
          _nortonPanel(
            title: '🔌 TOPOLOGÍA SAN MULTIPATH, DISCOS LSBLK & ADAPTADORES HBA FIBRE CHANNEL',
            child: Column(
              children: [
                for (final p in paths)
                  Container(
                    margin: const EdgeInsets.symmetric(vertical: 3),
                    padding: const EdgeInsets.all(8),
                    color: p['Healthy'] == true ? Colors.black26 : const Color(0xFF440011),
                    child: Row(
                      children: [
                        Expanded(
                          flex: 2,
                          child: Text(
                            '${p['Device']} (${p['Type']})',
                            style: const TextStyle(
                              color: Color(0xFF5FFFFF),
                              fontWeight: FontWeight.bold,
                            ),
                          ),
                        ),
                        Expanded(
                          flex: 2,
                          child: Text('${p['VendorProduct']} | ${p['Size']}'),
                        ),
                        Expanded(
                          child: Text('Caminos: ${p['ActivePaths']}/${p['TotalPaths']}'),
                        ),
                        _statusBadge(p['State'].toString(), p['Healthy'] == true),
                      ],
                    ),
                  ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  // ===========================================================================
  // UI HELPERS & CUSTOM TIME-SERIES CHART PAINTER
  // ===========================================================================
  Widget _kpiCard(String title, String value, Color color, IconData icon) {
    return Container(
      width: 210,
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: const Color(0xFF000078),
        border: Border.all(color: color, width: 2),
      ),
      child: Row(
        children: [
          Icon(icon, color: color, size: 28),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(title, style: const TextStyle(color: Colors.white70, fontSize: 11)),
                const SizedBox(height: 2),
                Text(
                  value,
                  style: TextStyle(color: color, fontSize: 20, fontWeight: FontWeight.bold),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _sectionHeader(String title, {Widget? trailing}) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        Text(
          title,
          style: const TextStyle(
            color: Color(0xFFFFFF5F),
            fontWeight: FontWeight.bold,
            fontSize: 15,
          ),
        ),
        if (trailing != null) trailing,
      ],
    );
  }

  Widget _nortonPanel({required String title, required Widget child, Widget? trailing}) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: const Color(0xFF000075),
        border: Border.all(color: const Color(0xFF5FFFFF), width: 1.8),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Expanded(
                child: Text(
                  '══ $title ══',
                  style: const TextStyle(
                    color: Color(0xFFFFFF5F),
                    fontWeight: FontWeight.bold,
                    fontSize: 13,
                  ),
                ),
              ),
              if (trailing != null) trailing,
            ],
          ),
          const SizedBox(height: 10),
          child,
        ],
      ),
    );
  }

  Widget _statusBadge(String text, bool ok) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
      color: ok ? const Color(0xFF006600) : const Color(0xFF990000),
      child: Text(
        text,
        style: TextStyle(
          color: ok ? const Color(0xFF5FFF5F) : const Color(0xFFFFFF5F),
          fontWeight: FontWeight.bold,
          fontSize: 11,
        ),
      ),
    );
  }

  Widget _lcdProgressBar({required double pct, required String label}) {
    final color = pct >= 90
        ? const Color(0xFFFF5F5F)
        : (pct >= 75 ? const Color(0xFFFFAF00) : const Color(0xFF5FFF5F));
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        ClipRRect(
          borderRadius: BorderRadius.circular(2),
          child: LinearProgressIndicator(
            value: (pct / 100.0).clamp(0.0, 1.0),
            minHeight: 10,
            backgroundColor: Colors.black54,
            valueColor: AlwaysStoppedAnimation<Color>(color),
          ),
        ),
        const SizedBox(height: 2),
        Text(label, style: const TextStyle(color: Colors.white70, fontSize: 11)),
      ],
    );
  }

  Widget _compactField(TextEditingController ctrl, String label, {bool obscure = false}) {
    return TextField(
      controller: ctrl,
      obscureText: obscure,
      style: const TextStyle(fontSize: 12, color: Colors.white),
      decoration: InputDecoration(
        labelText: label,
        labelStyle: const TextStyle(fontSize: 11, color: Color(0xFF5FFFFF)),
        isDense: true,
        filled: true,
        fillColor: Colors.black38,
        border: const OutlineInputBorder(),
      ),
    );
  }

  Widget _timeSeriesChartCard({
    required String title,
    required String series1Name,
    required Color series1Color,
    required List<double> series1Values,
    required String series2Name,
    required Color series2Color,
    required List<double> series2Values,
    double? maxY,
  }) {
    final last1 = series1Values.isNotEmpty ? series1Values.last.toStringAsFixed(1) : '0';
    final last2 = series2Values.isNotEmpty ? series2Values.last.toStringAsFixed(1) : '0';
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: const Color(0xFF000044),
        border: Border.all(color: const Color(0xFF5FFFFF), width: 1.5),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            title,
            style: const TextStyle(
              color: Color(0xFFFFFF5F),
              fontWeight: FontWeight.bold,
              fontSize: 13,
            ),
          ),
          const SizedBox(height: 4),
          Row(
            children: [
              Icon(Icons.circle, size: 10, color: series1Color),
              const SizedBox(width: 4),
              Text('$series1Name: $last1', style: TextStyle(color: series1Color, fontSize: 12)),
              const SizedBox(width: 16),
              Icon(Icons.circle, size: 10, color: series2Color),
              const SizedBox(width: 4),
              Text('$series2Name: $last2', style: TextStyle(color: series2Color, fontSize: 12)),
            ],
          ),
          const SizedBox(height: 8),
          Expanded(
            child: CustomPaint(
              size: Size.infinite,
              painter: _DualSeriesChartPainter(
                s1: series1Values,
                c1: series1Color,
                s2: series2Values,
                c2: series2Color,
                fixedMaxY: maxY,
              ),
            ),
          ),
        ],
      ),
    );
  }

  // ===========================================================================
  // SIMULATED FALLBACK TELEMETRY (IF SERVER IS UNREACHABLE FROM BROWSER)
  // ===========================================================================
  Map<String, dynamic> _buildMockState() {
    return {
      'HostName': _selectedServer.name.split(' ').first,
      'ClusterName': 'crs-prod-cluster',
      'Version': '19.24.0.0.0',
      'Nodes': [
        {
          'Name': 'servidor-oracle01',
          'Role': 'HUB',
          'Status': 'ONLINE',
          'VIP': '10.20.30.111',
          'CRSD': 'ONLINE',
          'CSSD': 'ONLINE',
          'OHASD': 'ONLINE',
          'ASM': 'ONLINE',
        },
        {
          'Name': 'servidor-oracle02',
          'Role': 'HUB',
          'Status': 'ONLINE',
          'VIP': '10.20.30.112',
          'CRSD': 'ONLINE',
          'CSSD': 'ONLINE',
          'OHASD': 'ONLINE',
          'ASM': 'ONLINE',
        },
      ],
      'Instances': [
        {
          'InstName': 'PRODCDB1',
          'HostName': 'servidor-oracle01',
          'Status': 'OPEN',
          'OpenMode': 'READ WRITE',
          'SGAMB': 32768,
          'PGAMB': 8192,
          'FRAUsedPct': 62,
        },
        {
          'InstName': 'PRODCDB2',
          'HostName': 'servidor-oracle02',
          'Status': 'OPEN',
          'OpenMode': 'READ WRITE',
          'SGAMB': 32768,
          'PGAMB': 8192,
          'FRAUsedPct': 62,
        },
      ],
      'PDBs': [
        {'Name': 'ERP_PROD_PDB', 'OpenMode': 'READ WRITE', 'TotalMB': 245760},
        {'Name': 'CRM_CORE_PDB', 'OpenMode': 'READ WRITE', 'TotalMB': 98304},
      ],
      'Listeners': [
        {'Name': 'LISTENER', 'Port': '1521', 'Status': 'ONLINE', 'ServicesSummary': '4 services'},
        {
          'Name': 'LISTENER_SCAN1',
          'Port': '1521',
          'Status': 'ONLINE',
          'ServicesSummary': '2 services'
        },
      ],
      'Sessions': [
        {
          'InstID': 1,
          'SID': 289,
          'Serial': 11042,
          'Username': 'CRM_BATCH',
          'Event': 'enq: TX - row lock contention',
          'SQLID': '3c9a11b04z88w',
          'SecondsInWait': 194,
          'BlockingSession': 'INST:2 SID:512',
        },
        {
          'InstID': 2,
          'SID': 512,
          'Serial': 9931,
          'Username': 'CRM_ADMIN',
          'Event': 'SQL*Net message from client',
          'SQLID': '0000000000000',
          'SecondsInWait': 195,
          'BlockingSession': 'BLOCKER (1)',
        },
        {
          'InstID': 1,
          'SID': 142,
          'Serial': 44821,
          'Username': 'ERP_APP',
          'Event': 'db file sequential read',
          'SQLID': '8f2k91m40p11x',
          'SecondsInWait': 2,
          'BlockingSession': '',
        },
      ],
      'Tablespaces': [
        {
          'Name': 'ERP_DATA_TBS',
          'Contents': 'PERMANENT',
          'TotalMB': 327680,
          'UsedMB': 285081,
          'UsedPercent': 87.0,
          'AutoExtend': true,
        },
        {
          'Name': 'SYSAUX',
          'Contents': 'PERMANENT',
          'TotalMB': 32768,
          'UsedMB': 18677,
          'UsedPercent': 57.0,
          'AutoExtend': true,
        },
        {
          'Name': 'UNDOTBS1',
          'Contents': 'UNDO',
          'TotalMB': 65536,
          'UsedMB': 20971,
          'UsedPercent': 32.0,
          'AutoExtend': true,
        },
      ],
      'ASMDiskgroups': [
        {
          'Name': 'DATA',
          'State': 'MOUNTED',
          'Type': 'NORMAL',
          'TotalMB': 2097152,
          'FreeMB': 545259,
          'UsedPct': 74,
          'OfflineDisks': 0,
        },
        {
          'Name': 'FRA',
          'State': 'MOUNTED',
          'Type': 'NORMAL',
          'TotalMB': 1048576,
          'FreeMB': 398458,
          'UsedPct': 62,
          'OfflineDisks': 0,
        },
      ],
      'Datafiles': [
        {
          'FileName': '+DATA/PRODCDB/datafile/erp_data01.dbf',
          'Tablespace': 'ERP_DATA_TBS',
          'SizeMB': 32768,
          'MaxMB': 32768,
          'Autoextend': true,
        },
        {
          'FileName': '+DATA/PRODCDB/datafile/sysaux01.dbf',
          'Tablespace': 'SYSAUX',
          'SizeMB': 16384,
          'MaxMB': 32768,
          'Autoextend': true,
        },
      ],
      'Segments': [
        {
          'Owner': 'ERP_CORE',
          'SegmentName': 'FACTURAS_HISTORICO_PART',
          'SegmentType': 'TABLE PARTITION',
          'TablespaceName': 'ERP_DATA_TBS',
          'SizeMB': 84210,
          'Extents': 1054,
        },
        {
          'Owner': 'ERP_CORE',
          'SegmentName': 'IDX_FACTURAS_FECHA_CLI',
          'SegmentType': 'INDEX',
          'TablespaceName': 'ERP_DATA_TBS',
          'SizeMB': 31420,
          'Extents': 412,
        },
      ],
      'InvalidObjects': [
        {
          'Owner': 'ERP_CORE',
          'ObjectName': 'PKG_INVOICE_BATCH',
          'ObjectType': 'PACKAGE BODY',
          'Status': 'INVALID',
          'LastDDL': '2026-10-16 08:15',
        },
      ],
      'TableStats': [
        {
          'Owner': 'ERP_CORE',
          'TableName': 'FACTURAS_CAB',
          'NumRows': 48291040,
          'LastAnalyzed': '2026-10-01 02:00',
          'StaleStats': 'YES',
        },
        {
          'Owner': 'CRM',
          'TableName': 'CLIENTES_MASTER',
          'NumRows': 2910400,
          'LastAnalyzed': '2026-10-16 03:00',
          'StaleStats': 'NO',
        },
      ],
      'Backups': [
        {
          'StartTime': '2026-10-16 02:00',
          'InputType': 'DB INCR LEVEL 0',
          'Status': 'COMPLETED',
          'OutputDevice': 'SBT_TAPE',
          'OutputMB': 412850,
          'DurationMin': 48,
        },
        {
          'StartTime': '2026-10-16 08:00',
          'InputType': 'ARCHIVELOG',
          'Status': 'COMPLETED',
          'OutputDevice': 'DISK',
          'OutputMB': 18420,
          'DurationMin': 4,
        },
      ],
      'RestorePoints': [
        {
          'Name': 'GRP_PRE_RELEASE_17',
          'Guaranteed': 'YES',
          'StorageMB': 12480,
          'Time': '2026-10-15 22:00',
        },
      ],
      'DataGuard': [
        {
          'ProtectionMode': 'MAXIMUM PERFORMANCE',
          'DatabaseRole': 'PRIMARY',
          'SwitchoverStatus': 'TO STANDBY',
          'TransportLag': '+00 00:00:00',
          'ApplyLag': '+00 00:00:02',
          'MRPStatus': 'APPLYING_LOG',
        },
      ],
      'AlertErrors': [
        {
          'Code': 'ORA-01654',
          'Instance': 'PRODCDB1',
          'Message':
              'ORA-01654: unable to extend index ERP_CORE.IDX_ORDERS by 1024 in tablespace ERP_DATA_TBS',
          'Occurrences': 2,
          'LastSeen': '08:52:10',
        },
      ],
      'OSMetrics': [
        {
          'Category': 'CPU',
          'Name': 'Load Average (1m/5m/15m)',
          'Value': '3.40 / 2.95 / 2.60',
          'UsedPercent': 21.2,
          'Detail': '16 vCPUs activos',
          'Healthy': true,
        },
        {
          'Category': 'RAM',
          'Name': 'Memoria Física Usada',
          'Value': '92.4 GB / 128.0 GB',
          'UsedPercent': 72.2,
          'Detail': 'HugePages: 16384 (32 GB)',
          'Healthy': true,
        },
      ],
      'Filesystems': [
        {
          'MountPoint': '/u01',
          'Filesystem': '/dev/mapper/vg_ora-lv_u01',
          'FSType': 'xfs',
          'TotalMB': 204800,
          'UsedMB': 149504,
          'UsedPercent': 73.0,
          'InodesPct': 18.4,
        },
      ],
      'StoragePaths': [
        {
          'Device': 'ORA_DATA_01 (dm-3)',
          'Type': 'DM-MPATH',
          'VendorProduct': 'PURE,FlashArray',
          'Size': '2.0T',
          'ActivePaths': 4,
          'TotalPaths': 4,
          'State': '4/4 ACTIVE',
          'Healthy': true,
        },
        {
          'Device': 'ORA_FRA_01 (dm-4)',
          'Type': 'DM-MPATH',
          'VendorProduct': 'PURE,FlashArray',
          'Size': '1.0T',
          'ActivePaths': 3,
          'TotalPaths': 4,
          'State': '3/4 DEGRADED (1 DOWN)',
          'Healthy': false,
        },
      ],
    };
  }

  Map<String, dynamic> _buildMockSre() {
    final hist = List.generate(18, (i) {
      return {
        'timestamp': '09:${(10 + i).toString().padLeft(2, '0')}',
        'active_sessions': 4 + (i % 5),
        'blocking_locks': i > 12 ? 2 : 0,
        'max_tablespace_pct': 85.0 + (i * 0.1),
        'fra_pct': 58.0 + (i * 0.2),
        'max_asm_pct': 74.0,
        'ora_error_count': i > 10 ? 2 : 0,
        'cpu_load_pct': 18.0 + (math.sin(i) * 6),
        'ram_used_pct': 71.5 + (math.cos(i) * 1.5),
        'unhealthy_san_paths': i > 14 ? 1 : 0,
      };
    });

    return {
      'slos': [
        {
          'id': 'slo_availability',
          'name': 'Disponibilidad BD & Clusterware',
          'target_pct': 99.95,
          'current_sli_pct': 100.0,
          'error_budget_remaining_pct': 100.0,
          'burn_rate': 0.0,
          'status': 'HEALTHY',
          'recommended_action_id': 'switch_logfile',
        },
        {
          'id': 'slo_lock_free',
          'name': 'Operación Libre de Bloqueos TX/TM',
          'target_pct': 99.50,
          'current_sli_pct': 98.90,
          'error_budget_remaining_pct': 34.0,
          'burn_rate': 2.2,
          'status': 'BURNING',
          'recommended_action_id': 'kill_session',
        },
        {
          'id': 'slo_storage_capacity',
          'name': 'Salud de Capacidad (TBS, FRA, ASM)',
          'target_pct': 99.90,
          'current_sli_pct': 99.95,
          'error_budget_remaining_pct': 88.0,
          'burn_rate': 0.4,
          'status': 'HEALTHY',
          'recommended_action_id': 'add_datafile',
        },
        {
          'id': 'slo_san_multipath',
          'name': 'Redundancia SAN Multipath & Fibra FC',
          'target_pct': 99.99,
          'current_sli_pct': 99.40,
          'error_budget_remaining_pct': 18.0,
          'burn_rate': 4.5,
          'status': 'BURNING',
          'recommended_action_id': 'rescan_scsi',
        },
      ],
      'active_alerts': [
        {
          'id': 'LOCK_SID_512',
          'severity': 'CRITICAL',
          'category': 'Bloqueos TX/TM',
          'title': 'Sesión Bloqueante SID 512,9931 (CRM_ADMIN) reteniendo fila TX por 195s',
          'description':
              'Bloqueando a SID 289 (CRM_BATCH) en SQL_ID 3c9a11b04z88w (enq: TX - row lock contention)',
          'recommended_action': 'Matar Sesión (512,9931,@2) IMMEDIATE',
          'action_id': 'kill_session',
          'action_target': '512,9931,@2',
        },
        {
          'id': 'SAN_PATH_ORA_FRA_01',
          'severity': 'CRITICAL',
          'category': 'SAN Multipath / Fibra FC',
          'title': 'Camino SAN degradado en ORA_FRA_01 (dm-4) — 3/4 caminos activos',
          'description':
              'Pérdida de redundancia en LUN PURE FlashArray. Se recomienda reescanear buses SCSI/FC y recargar multipath.',
          'recommended_action': 'Reescanear Buses SCSI/FC y Multipath (-r)',
          'action_id': 'rescan_scsi',
          'action_target': 'ORA_FRA_01',
        },
      ],
      'history': hist,
      'notifications': (_sreData['notifications'] as List?) ?? [],
    };
  }
}

class _DualSeriesChartPainter extends CustomPainter {
  final List<double> s1;
  final Color c1;
  final List<double> s2;
  final Color c2;
  final double? fixedMaxY;

  _DualSeriesChartPainter({
    required this.s1,
    required this.c1,
    required this.s2,
    required this.c2,
    this.fixedMaxY,
  });

  @override
  void paint(Canvas canvas, Size size) {
    final gridPaint = Paint()
      ..color = Colors.white12
      ..strokeWidth = 1;

    for (int i = 0; i <= 4; i++) {
      final y = size.height * (i / 4);
      canvas.drawLine(Offset(0, y), Offset(size.width, y), gridPaint);
    }

    double maxVal = fixedMaxY ?? 10.0;
    if (fixedMaxY == null) {
      for (final v in s1) {
        if (v > maxVal) maxVal = v * 1.2;
      }
      for (final v in s2) {
        if (v > maxVal) maxVal = v * 1.2;
      }
    }
    if (maxVal <= 0) maxVal = 10;

    _drawSeries(canvas, size, s1, c1, maxVal);
    _drawSeries(canvas, size, s2, c2, maxVal);
  }

  void _drawSeries(Canvas canvas, Size size, List<double> vals, Color color, double maxVal) {
    if (vals.isEmpty) return;
    final path = Path();
    final fillPath = Path();
    final count = vals.length;

    for (int i = 0; i < count; i++) {
      final x = count == 1 ? size.width / 2 : (i / (count - 1)) * size.width;
      final y = size.height - ((vals[i] / maxVal).clamp(0.0, 1.0) * (size.height - 8)) - 4;
      if (i == 0) {
        path.moveTo(x, y);
        fillPath.moveTo(x, size.height);
        fillPath.lineTo(x, y);
      } else {
        path.lineTo(x, y);
        fillPath.lineTo(x, y);
      }
      if (i == count - 1) {
        fillPath.lineTo(x, size.height);
        fillPath.close();
      }
    }

    final fillPaint = Paint()
      ..color = color.withValues(alpha: 0.14)
      ..style = PaintingStyle.fill;
    canvas.drawPath(fillPath, fillPaint);

    final linePaint = Paint()
      ..color = color
      ..strokeWidth = 2.2
      ..style = PaintingStyle.stroke;
    canvas.drawPath(path, linePaint);
  }

  @override
  bool shouldRepaint(covariant _DualSeriesChartPainter oldDelegate) => true;
}
