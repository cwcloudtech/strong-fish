import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:open_filex/open_filex.dart';
import 'package:share_plus/share_plus.dart';

import '../providers/providers.dart';

/// The formats a program is exported in.
///
/// The same set the web offers, and they are not the same document by
/// accident: the PDF is what a coach prints and takes to the gym, the XLSX is
/// what an athlete fills in on a laptop and mails back, and the image is what
/// either of them pastes into a conversation - one picture, nothing to open.
enum ExportFormat {
  pdf('pdf'),
  xlsx('xlsx'),
  png('png');

  const ExportFormat(this.extension);
  final String extension;
}

/// "Export" opening the formats.
///
/// A menu rather than a row of buttons: exporting happens rarely, and the
/// slots in an app bar are ones the sessions themselves do not get. Mirrors
/// the web's ExportMenu.
class SfExportButton extends ConsumerWidget {
  final Future<void> Function(ExportFormat) onExport;
  final bool busy;
  final String? tooltip;
  final IconData icon;

  const SfExportButton({
    super.key,
    required this.onExport,
    this.busy = false,
    this.tooltip,
    this.icon = Icons.download_outlined,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final t = ref.watch(tProvider);

    if (busy) {
      return const Padding(
        padding: EdgeInsets.all(14),
        child: SizedBox(width: 18, height: 18, child: CircularProgressIndicator(strokeWidth: 2)),
      );
    }

    return PopupMenuButton<ExportFormat>(
      icon: Icon(icon),
      tooltip: tooltip ?? t('programs.export'),
      onSelected: onExport,
      itemBuilder: (context) => [
        PopupMenuItem(
          value: ExportFormat.pdf,
          child: Row(children: [
            const Icon(Icons.picture_as_pdf_outlined, size: 18),
            const SizedBox(width: 10),
            Text(t('programs.exportPdf')),
          ]),
        ),
        PopupMenuItem(
          value: ExportFormat.xlsx,
          child: Row(children: [
            const Icon(Icons.grid_on_outlined, size: 18),
            const SizedBox(width: 10),
            Text(t('programs.exportXlsx')),
          ]),
        ),
        PopupMenuItem(
          value: ExportFormat.png,
          child: Row(children: [
            const Icon(Icons.image_outlined, size: 18),
            const SizedBox(width: 10),
            Text(t('programs.exportPng')),
          ]),
        ),
      ],
    );
  }
}

/// Hands over a file that was just downloaded, and says so when nothing can
/// take it.
///
/// A document is opened; an image is shared. The picture exists to be sent to
/// somebody - opening it in a gallery is a detour through the one app the
/// person is not trying to reach - so it goes straight to the share sheet,
/// where the conversation they had in mind already is.
///
/// A phone with no PDF viewer - or, more often, no spreadsheet app - is a real
/// case, and "nothing happened" is the worst way to find out.
Future<void> openExported(BuildContext context, WidgetRef ref, String path) async {
  final messenger = ScaffoldMessenger.of(context);
  final t = ref.read(tProvider);

  if (path.toLowerCase().endsWith('.png')) {
    // The origin rect is what anchors the popover on iPad; without it the
    // sheet opens in the corner.
    final box = context.findRenderObject() as RenderBox?;
    await SharePlus.instance.share(ShareParams(
      files: [XFile(path, mimeType: 'image/png')],
      sharePositionOrigin:
          box != null && box.hasSize ? box.localToGlobal(Offset.zero) & box.size : null,
    ));
    return;
  }

  final result = await OpenFilex.open(path);
  if (result.type != ResultType.done) {
    messenger.showSnackBar(SnackBar(content: Text(t('programs.exportNoViewer'))));
  }
}
