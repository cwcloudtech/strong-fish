import { useMemo, useState } from "react";
import SelectPanel from "./SelectPanel";
import { useI18n } from "../../i18n/I18nContext";

/**
 * Autocomplete multi-select, ported from ~/cwclock: a search box narrows the
 * list, checkboxes toggle, and the panel stays open across toggles so several
 * options can be picked in a row.
 *
 * Options are `{ value, label }`. `selected` is an array of values.
 */
export default function MultiSelect({
  options,
  selected,
  onChange,
  label,
  placeholder,
  disabled,
  id,
  className,
}) {
  const { t } = useI18n();
  const [query, setQuery] = useState("");

  const filtered = useMemo(() => {
    const needle = query.trim().toLowerCase();
    if (!needle) return options;
    return options.filter((option) => option.label.toLowerCase().includes(needle));
  }, [options, query]);

  const toggle = (value) => {
    onChange(selected.includes(value) ? selected.filter((each) => each !== value) : [...selected, value]);
  };

  // Selects everything currently *filtered*, merged with what is already
  // selected outside the search - so typing a query and then "select all"
  // picks that subset rather than the lot.
  const selectAllFiltered = () => {
    onChange(Array.from(new Set([...selected, ...filtered.map((option) => option.value)])));
  };
  const allFilteredSelected = filtered.length > 0 && filtered.every((o) => selected.includes(o.value));

  // One name reads better than "1 selected", and past that a count is the only
  // thing that fits.
  const summary =
    selected.length === 0
      ? ""
      : selected.length === 1
        ? options.find((option) => option.value === selected[0])?.label || t("common.selectedOne")
        : t("common.selectedCount", { count: selected.length });

  return (
    <SelectPanel
      id={id}
      className={className}
      label={label}
      summary={summary}
      placeholder={placeholder || t("common.selectPlaceholder")}
      disabled={disabled}
      searchPlaceholder={t("common.search")}
      query={query}
      onQueryChange={setQuery}
    >
      {() => (
        <>
          <div className="sf-select-list" role="listbox" aria-multiselectable="true">
            {filtered.length === 0 ? <p className="sf-select-empty">{t("common.noResults")}</p> : null}
            {filtered.map((option) => (
              <label key={option.value} className="sf-select-option">
                <input
                  type="checkbox"
                  checked={selected.includes(option.value)}
                  onChange={() => toggle(option.value)}
                />
                <span>{option.label}</span>
              </label>
            ))}
          </div>

          {(!allFilteredSelected && filtered.length > 0) || selected.length > 0 ? (
            <div className="sf-select-actions">
              {!allFilteredSelected && filtered.length > 0 ? (
                <button type="button" className="sf-select-action" onClick={selectAllFiltered}>
                  {t("common.selectAll")}
                </button>
              ) : null}
              {selected.length > 0 ? (
                <button type="button" className="sf-select-action" onClick={() => onChange([])}>
                  {t("common.clearSelection")}
                </button>
              ) : null}
            </div>
          ) : null}
        </>
      )}
    </SelectPanel>
  );
}
