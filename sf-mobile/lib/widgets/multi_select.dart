import 'package:flutter/material.dart';

/// One option in [SfMultiSelect].
class SelectOption {
  final String value;
  final String label;

  const SelectOption({required this.value, required this.label});
}

/// Autocomplete multi-select: a search box narrows the list, checkboxes toggle,
/// and the sheet stays open across toggles so several can be picked in a row.
///
/// The same behaviour as the web app's MultiSelect (itself a port of
/// ~/cwclock's), shaped for a phone: opened as a full-height sheet rather than
/// a dropdown panel, because a list of a club's members does not fit in one.
///
/// Shown as a field that reads back what is picked, so the screen behind it
/// stays legible while nothing is selected.
class SfMultiSelect extends StatelessWidget {
  final String label;
  final String placeholder;
  final List<SelectOption> options;
  final List<String> selected;
  final ValueChanged<List<String>> onChanged;
  final String searchHint;
  final String noResults;
  final String selectedCount;
  final String clearLabel;

  const SfMultiSelect({
    super.key,
    required this.label,
    required this.placeholder,
    required this.options,
    required this.selected,
    required this.onChanged,
    required this.searchHint,
    required this.noResults,
    required this.selectedCount,
    required this.clearLabel,
  });

  @override
  Widget build(BuildContext context) {
    // One name reads better than "1 selected"; past that a count is all that
    // fits on a phone's width.
    final summary = switch (selected.length) {
      0 => placeholder,
      1 => options
          .where((option) => option.value == selected.first)
          .map((option) => option.label)
          .firstOrNull ??
          placeholder,
      _ => selectedCount.replaceAll('{{count}}', '${selected.length}'),
    };

    return InputDecorator(
      decoration: InputDecoration(labelText: label),
      child: InkWell(
        onTap: () async {
          final picked = await showModalBottomSheet<List<String>>(
            context: context,
            isScrollControlled: true,
            showDragHandle: true,
            builder: (sheetContext) => _MultiSelectSheet(
              options: options,
              selected: selected,
              searchHint: searchHint,
              noResults: noResults,
              clearLabel: clearLabel,
            ),
          );
          if (picked != null) onChanged(picked);
        },
        child: Row(
          children: [
            Expanded(
              child: Text(
                summary,
                overflow: TextOverflow.ellipsis,
                style: selected.isEmpty
                    ? TextStyle(color: Theme.of(context).hintColor)
                    : const TextStyle(fontWeight: FontWeight.w600),
              ),
            ),
            const Icon(Icons.expand_more, size: 20),
          ],
        ),
      ),
    );
  }
}

class _MultiSelectSheet extends StatefulWidget {
  final List<SelectOption> options;
  final List<String> selected;
  final String searchHint;
  final String noResults;
  final String clearLabel;

  const _MultiSelectSheet({
    required this.options,
    required this.selected,
    required this.searchHint,
    required this.noResults,
    required this.clearLabel,
  });

  @override
  State<_MultiSelectSheet> createState() => _MultiSelectSheetState();
}

class _MultiSelectSheetState extends State<_MultiSelectSheet> {
  late List<String> _picked = List<String>.from(widget.selected);
  final _query = TextEditingController();

  @override
  void dispose() {
    _query.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final needle = _query.text.trim().toLowerCase();
    final filtered = needle.isEmpty
        ? widget.options
        : widget.options.where((option) => option.label.toLowerCase().contains(needle)).toList();

    return Padding(
      // Above the keyboard: the search field is the first thing tapped, and a
      // list hidden behind the keyboard is a list nobody can pick from.
      padding: EdgeInsets.only(bottom: MediaQuery.of(context).viewInsets.bottom),
      child: SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 16),
              child: TextField(
                controller: _query,
                autofocus: true,
                decoration: InputDecoration(
                  hintText: widget.searchHint,
                  prefixIcon: const Icon(Icons.search),
                ),
                onChanged: (_) => setState(() {}),
              ),
            ),
            Flexible(
              child: filtered.isEmpty
                  ? Padding(
                      padding: const EdgeInsets.all(24),
                      child: Text(widget.noResults),
                    )
                  : ListView.builder(
                      shrinkWrap: true,
                      itemCount: filtered.length,
                      itemBuilder: (context, index) {
                        final option = filtered[index];
                        return CheckboxListTile(
                          dense: true,
                          title: Text(option.label),
                          value: _picked.contains(option.value),
                          onChanged: (checked) => setState(() {
                            if (checked == true) {
                              _picked.add(option.value);
                            } else {
                              _picked.remove(option.value);
                            }
                          }),
                        );
                      },
                    ),
            ),
            Padding(
              padding: const EdgeInsets.all(16),
              child: Row(
                children: [
                  if (_picked.isNotEmpty)
                    TextButton(
                      onPressed: () => setState(() => _picked = []),
                      child: Text(widget.clearLabel),
                    ),
                  const Spacer(),
                  FilledButton(
                    onPressed: () => Navigator.of(context).pop(_picked),
                    child: Text(MaterialLocalizations.of(context).okButtonLabel),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
