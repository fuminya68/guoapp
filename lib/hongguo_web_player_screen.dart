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
  bool _externalBrowserMode = false;
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

    try {
      final installed = await WebviewController.getWebViewVersion();
      if (installed == null) {
        throw StateError('找不到 Microsoft Edge WebView2 Runtime');
      }
      _environmentReady ??= _configureEnvironment();
      await _environmentReady;
      if (!_permitted) return;
      await _navigateToEpisode(_index);
    } catch (_) {
      if (mounted && !_closed) {
        setState(() => _error = 'WebView2 無法啟動。請改用外部瀏覽器觀看。');
      }
    }
  }

  /// Each episode gets a fresh WebView2 controller so stale player media,
  /// route handlers and decoder instances cannot leak into the next episode.
  /// The shared WebView2 user-data directory retains allowed website sessions.
  Future<void> _navigateToEpisode(int index) async {
    if (!_permitted || index < 0 || index >= widget.detail.episodes.length) return;
    final url = publicHongguoEpisodeUrl(
      widget.detail.drama,
      widget.detail.episodes[index],
    );
    if (url == null || !isTrustedHongguoWebviewNavigation(url)) {
      if (mounted) setState(() => _error = '本集網址無效，已停止切換。');
      return;
    }

    final ticket = ++_generation;
    setState(() {
      _index = index;
      _error = null;
      _ready = false;
    });

    await _urlSubscription?.cancel();
    await _errorSubscription?.cancel();
    _urlSubscription = null;
    _errorSubscription = null;
    final previous = _controller;
    _controller = null;
    if (previous != null) {
      try {
        await previous.stop();
        await previous.dispose();
      } catch (_) {
        // A failed close must not authorize an unexpected website.
      }
    }
    if (!_permitted || ticket != _generation) return;
    if (_externalBrowserMode) {
      // Only a user-selected mode may open the browser automatically when
      // navigating episodes. The browser retains its own authorized session;
      // no cookies or account material are accessed by the app.
      final opened = await openPublicHongguoEpisodeInBrowser(
        widget.detail.drama,
        widget.detail.episodes[index],
      );
      if (!opened && _permitted && ticket == _generation) {
        setState(() => _error = '無法在系統瀏覽器開啟這一集。');
      }
      return;
    }

    WebviewController? next;
    try {
      next = WebviewController();
      _controller = next;
      await next.initialize();
      if (!_permitted || ticket != _generation) return;

      await next.setPopupWindowPolicy(WebviewPopupWindowPolicy.deny);
      await next.setDevToolsEnabled(false);
      await next.setDefaultContextMenusEnabled(false);
      final script = hongguoSameSeriesNavigationScript(widget.detail.drama);
      if (script != null) {
        await next.addScriptToExecuteOnDocumentCreated(script);
      }

      _urlSubscription = next.url.listen((address) {
        if (_closed || _controller != next || ticket != _generation) return;
        if (address == 'about:blank') return;
        if (!isTrustedHongguoWebviewNavigation(address)) {
          unawaited(next!.stop());
          if (mounted) {
            setState(() => _error = '網頁導向了未受信任的網站，已停止播放。');
          }
          return;
        }
        final selected = hongguoEpisodeIndexFromWebviewUrl(
          widget.detail,
          address,
        );
        if (selected != null && selected != _index && mounted) {
          setState(() => _index = selected);
        }
      });
      _errorSubscription = next.onLoadError.listen((error) {
        if (mounted && !_closed && _controller == next) {
          setState(() => _error = '網頁載入失敗，可以改用外部瀏覽器觀看本集。');
        }
      });
      if (!_permitted || ticket != _generation) return;
      setState(() => _ready = true);
      await next.loadUrl(url);
    } catch (_) {
      if (mounted && !_closed && ticket == _generation) {
        setState(() {
          _ready = false;
          _error = '無法開啟本集網頁，請改用外部瀏覽器。';
        });
      }
    }
  }

  Future<void> _chooseEpisode() async {
    if (!_permitted) return;
    final chosen = await showModalBottomSheet<int>(
      context: context,
      isScrollControlled: true,
      builder: (context) => SafeArea(
        child: SizedBox(
          height: MediaQuery.sizeOf(context).height * 0.7,
          child: Column(
            children: [
              const ListTile(title: Text('選擇集數')),
              Expanded(
                child: ListView.builder(
                  itemCount: widget.detail.episodes.length,
                  itemBuilder: (context, index) {
                    final episode = widget.detail.episodes[index];
                    final supported = publicHongguoEpisodeUrl(
                      widget.detail.drama, episode,
                    ) != null;
                    return ListTile(
                      key: ValueKey('hongguo-episode-$index'),
                      title: Text(episode.title),
                      selected: index == _index,
                      enabled: supported,
                      onTap: supported ? () => Navigator.pop(context, index) : null,
                    );
                  },
                ),
              ),
            ],
          ),
        ),
      ),
    );
    if (chosen != null && mounted && _permitted && chosen != _index) {
      await _navigateToEpisode(chosen);
    }
  }

  Future<void> _toggleBrowserMode() async {
    if (!_permitted) return;
    setState(() => _externalBrowserMode = !_externalBrowserMode);
    // Refresh the current episode into the selected player. When switching
    // to the system browser, the embedded WebView2 is disposed first.
    await _navigateToEpisode(_index);
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
            tooltip: '選擇集數',
            onPressed: _chooseEpisode,
            icon: const Icon(Icons.format_list_numbered),
          ),
          IconButton(
            key: const ValueKey('hongguo-toggle-system-browser'),
            tooltip: _externalBrowserMode
                ? '改回內嵌播放器'
                : '後續集數使用系統瀏覽器',
            onPressed: _toggleBrowserMode,
            icon: Icon(
              _externalBrowserMode ? Icons.web_rounded : Icons.language_rounded,
            ),
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
          if (!_externalBrowserMode && _ready && _controller != null)
            Positioned.fill(
              child: Webview(
                _controller!,
                permissionRequested: (url, permission, userInitiated) =>
                    WebviewPermissionDecision.deny,
              ),
            ),
          if (_externalBrowserMode && _error == null)
            Center(
              child: Padding(
                padding: const EdgeInsets.all(24),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    const Icon(Icons.open_in_browser_rounded, size: 48),
                    const SizedBox(height: 16),
                    const Text(
                      '目前使用系統瀏覽器觀看紅果',
                      textAlign: TextAlign.center,
                    ),
                    const SizedBox(height: 8),
                    const Text(
                      '上一集、下一集及選擇集數會在你既有的瀏覽器中開啟。'
                      '瀏覽器的登入狀態不會匯入真果鑑。',
                      textAlign: TextAlign.center,
                    ),
                    const SizedBox(height: 20),
                    FilledButton.icon(
                      onPressed: _toggleBrowserMode,
                      icon: const Icon(Icons.web_rounded),
                      label: const Text('改回內嵌播放器'),
                    ),
                  ],
                ),
              ),
            ),
          if (!_externalBrowserMode && !_ready && _error == null)
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
