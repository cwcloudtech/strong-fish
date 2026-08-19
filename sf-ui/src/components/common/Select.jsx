import { useMemo, useState } from "react";
import { FiCheck } from "react-icons/fi";
import SelectPanel from "./SelectPanel";
import { useI18n } from "../../i18n/I18nContext";

/**
 * Single-select autocomplete, ported from ~/cwclock's AutocompleteSelect:
 * the same trigger and panel as MultiSelect, but picking an option selects it
 * and closes the panel instead of staying open for several.
 *
 * Replaces the app's plain `<select class="sf-select">` fields. A native select
 * cannot be given this layout, cannot be searched, and renders as the operating
 * system's own widget - which is the thing that looked out of place beside the
 * rest of the design.
 *
 * The search box only appears once there are enough options to be worth
 * searching: on a three-item field it is furniture.
 */

/** Below this, the list is short enough to read at a glance. */
const SEARCHABLE_FROM = 7;

export default function Select({
  options,
  value,
  onChange,
  label,
  placeholder,
  disabled,
  clearable = false,
  id,
  className,
}) {
  const { t } = useI18n();
  const [query, setQuery] = useState("");
  const searchable = options.length >= SEARCHABLE_FROM;

  const filtered = useMemo(() => {
    const needle = query.trim().toLowerCase();
    if (!needle) return options;
    return options.filter((option) => option.label.toLowerCase().includes(needle));
  }, [options, query]);

  const selected = options.find((option) => option.value === value);

  return (
    <SelectPanel
      id={id}
      className={className}
      label={label}
      summary={selected?.label || ""}
      placeholder={placeholder || t("common.selectPlaceholder")}
      disabled={disabled}
      searchPlaceholder={t("common.search")}
      query={query}
      onQueryChange={searchable ? setQuery : undefined}
    >
      {(close) => (
        <>
          <div className="sf-select-list" role="listbox">
            {filtered.length === 0 ? <p className="sf-select-empty">{t("common.noResults")}</p> : null}
            {filtered.map((option) => (
              <button
                key={option.value}
                type="button"
                role="option"
                aria-selected={option.value === value}
                className={`sf-select-option is-button ${option.value === value ? "active" : ""}`}
                onClick={() => {
                  onChange(option.value);
                  close();
                }}
              >
                <span>{option.label}</span>
                {option.value === value ? <FiCheck aria-hidden="true" /> : null}
              </button>
            ))}
          </div>

          {clearable && value ? (
            <div className="sf-select-actions">
              <button
                type="button"
                className="sf-select-action"
                onClick={() => {
                  onChange("");
                  close();
                }}
              >
                {t("common.clearSelection")}
              </button>
            </div>
          ) : null}
        </>
      )}
    </SelectPanel>
  );
}
