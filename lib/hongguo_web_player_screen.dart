import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:path/path.dart' as p;
import 'package:path_provider/path_provider.dart';
import 'package:webview_flutter_windows/webview_flutter_windows.dart';

import 'hongguo_web_fallback.dart';
import 'local_store.dart';
import 'models.dart';

/// WebView2 preserves the website's own browser playback pipeline.
/// No native media URLs, keys, cookies, or injected scripts are extracted.
class HongguoWebPlayerScreen extends StatefulWidget {
  const HongguoWebPlayerScreen({
    super.key,
    required this.detail,
    required this.initialIndex,
    required this.store,
  });

  final DramaDetail detail;
  final int initialIndex;
  final LocalStore store;

  @override
  State<HongguoWebPlayerScreen> createState() => _HongguoWebPlayerScreenState();
}

class _HongguoWebPlayerScreenState extends State<HongguoWebPlayerScreen> {
  static Future<void>? _environmentReady;

  WebviewController? _controller;
  StreamSubscription<String>? _urlSubscription;
  StreamSubscription<WebErrorStatus>? _errorSubscription;
  late final int _epoch = widget.store.profileEpoch;
  late int _index = widget.initialIndex;
  bool _ready = false;
  bool _closed = false;
  String? _error;
  int _generation = 0;

  @override
  void initState() {
    super.initState();
    widget.store.addListener(_enforceProfileAccess);
    unawaited(_initialize());
  }

  static Future<void> _configureEnvironment() async {
    final appData = await getApplicationSupportDirectory();
    await WebviewController.initializeEnvironment(
      userDataPath: p.join(appData.path, 'webview2-hongguo'),
    );
  }

  bool get _permitted =>
      !_closed &&
      mounted &&
      !widget.store.locked &&
      widget.store.profileEpoch == _epoch &&
      widget.store.allowsSource(widget.detail.drama.source);

  void _enforceProfileAccess() {
    if (_closed || !mounted || _permitted) return;
    unawaited(Navigator.of(context).maybePop());
  }

  Future<void> _initialize() async {
    if (!Platform.isWindows ||
        !supportsHongguoInAppWebPlayback(
          windows: true,
          detail: widget.detail,
          episodeIndex: _index,
        ) ||
        !_permitted) {
      if (mounted) setState(() => _error = '此集無法使用 Windows 網頁播放器');
      return;
    }
    WebviewController? controller;
    try {
      final installed = await WebviewController.getWebViewVersion();
      if (installed == null) {
        throw StateError('找不到 Microsoft Edge WebView2 Runtime，請使用外部瀏覽器觀看。');
      }
      _environmentReady ??= _configureEnvironment();
      await _environmentReady;
      if (!_permitted) return;

      controller = WebviewController();
      _controller = controller;
      await controller.initialize();
      if (!_permitted) return;

      await controller.setPopupWindowPolicy(WebviewPopupWindowPolicy.deny);
      await controller.setDevToolsEnabled(false);
      await controller.setDefaultContextMenusEnabled(false);

      _urlSubscription = controller.url.listen((url) {
        if (_closed || isTrustedHongguoWebviewNavigation(url)) return;
        // Refuse an unexpected top-level redirect instead of following it.
        unawaited(controller!.stop());
        if (mounted) {
          setState(() => _error = '網頁導向了未受信任的網站，已停止內嵌播放。');
        }
      });
      _errorSubscription = controller.onLoadError.listen((error) {
        if (mounted && !_closed) {
          setState(() => _error = '網頁載入失敗。可改用外部瀏覽器播放本集。');
        }
      });

      if (!_permitted) return;
      setState(() => _ready = true);
      await _navigateToEpisode(_index);
    } catch (_) {
      if (mounted && !_closed) {
        setState(() => _error = 'WebView2 無法啟動。可以使用外部瀏覽器開啟本集。');
      }
    }
  }

  Future<void> _navigateToEpisode(int index) async {
    if (!_permitted || index < 0 || index >= widget.detail.episodes.length) return;
    final url = publicHongguoEpisodeUrl(
      widget.detail.drama,
      widget.detail.episodes[index],
    );
    if (url == null || !isTrustedHongguoWebviewNavigation(url)) return;
    final ticket = ++_generation;
    setState(() {
      _index = index;
      _error = null;
    });
    final controller = _controller;
    if (controller == null || !controller.value.isInitialized) return;
    try {
      await controller.loadUrl(url);
    } catch (_) {
      if (mounted && !_closed && ticket == _generation) {
        setState(() => _error = '無法載入本集網頁，請改用外部瀏覽器。');
      }
    }
  }

  Future<void> _openExternal() async {
    if (!_permitted || _index >= widget.detail.episodes.length) return;
    final ok = await openPublicHongguoEpisodeInBrowser(
      widget.detail.drama,
      widget.detail.episodes[_index],
    );
    if (!ok && mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('無法開啟外部瀏覽器')),
      );
    }
  }

  @override
  void dispose() {
    _closed = true;
    _generation++;
    widget.store.removeListener(_enforceProfileAccess);
    unawaited(_urlSubscription?.cancel() ?? Future<void>.value());
    unawaited(_errorSubscription?.cancel() ?? Future<void>.value());
    final controller = _controller;
    if (controller != null) unawaited(controller.dispose());
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final current = _index >= 0 && _index < widget.detail.episodes.length
        ? widget.detail.episodes[_index]
        : null;
    return Scaffold(
      appBar: AppBar(
        title: Text('${widget.detail.drama.title} · 第${current?.number ?? 1}集'),
        actions: [
          IconButton(
            tooltip: '上一集',
            onPressed: _index > 0 ? () => _navigateToEpisode(_index - 1) : null,
            icon: const Icon(Icons.skip_previous),
          ),
          IconButton(
            tooltip: '下一集',
            onPressed: _index + 1 < widget.detail.episodes.length
                ? () => _navigateToEpisode(_index + 1)
                : null,
            icon: const Icon(Icons.skip_next),
          ),
          IconButton(
            tooltip: '用外部瀏覽器開啟本集',
            onPressed: _openExternal,
            icon: const Icon(Icons.open_in_browser),
          ),
        ],
      ),
      body: Stack(
        children: [
          if (_ready && _controller != null)
            Positioned.fill(
              child: Webview(
                _controller!,
                permissionRequested: (url, permission, userInitiated) =>
                    WebviewPermissionDecision.deny,
              ),
            ),
          if (!_ready && _error == null)
            const Center(child: CircularProgressIndicator()),
          if (_error != null)
            Positioned.fill(
              child: ColoredBox(
                color: Theme.of(context).scaffoldBackgroundColor,
                child: Center(
                  child: Padding(
                    padding: const EdgeInsets.all(24),
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        const Icon(Icons.web_asset_off_outlined, size: 42),
                        const SizedBox(height: 16),
                        Text(_error!, textAlign: TextAlign.center),
                        const SizedBox(height: 20),
                        FilledButton.icon(
                          onPressed: _openExternal,
                          icon: const Icon(Icons.open_in_browser),
                          label: const Text('在瀏覽器觀看本集'),
                        ),
                      ],
                    ),
                  ),
                ),
              ),
            ),
        ],
      ),
    );
  }
}
