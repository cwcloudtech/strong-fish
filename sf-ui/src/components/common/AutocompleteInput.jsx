import { useRef, useState } from "react";

/**
 * A text field with suggestions, where anything may be typed.
 *
 * Not a Select: the list is a convenience, not the set of valid answers. The
 * model names come from the provider catalogue, a vendor ships one the day
 * before the table hears about it, and a field that refuses it would make
 * this app the reason somebody cannot use their own subscription.
 *
 * Hand-rolled rather than a native datalist, which browsers render in their
 * own widget - the same reason this app's Select exists (see SelectPanel).
 */
export default function AutocompleteInput({
  value,
  onChange,
  options = [],
  placeholder,
  id,
  inputMode,
  autoComplete = "off",
}) {
  const [open, setOpen] = useState(false);
  const blurTimer = useRef(null);

  const needle = (value || "").trim().toLowerCase();
  // Everything while the field holds exactly one of the options - having
  // picked "gpt-4o" should not hide "gpt-4o-mini".
  const matches = options.filter(
    (option) => !needle || option.toLowerCase() === needle || option.toLowerCase().includes(needle)
  );

  return (
    <div className="sf-autocomplete">
      <input
        className="sf-input"
        id={id}
        value={value}
        placeholder={placeholder}
        inputMode={inputMode}
        autoComplete={autoComplete}
        onChange={(event) => {
          onChange(event.target.value);
          setOpen(true);
        }}
        onFocus={() => setOpen(true)}
        // Deferred, or the click that picks a suggestion lands after the list
        // has already been unmounted by the blur.
        onBlur={() => {
          blurTimer.current = setTimeout(() => setOpen(false), 120);
        }}
        onKeyDown={(event) => {
          if (event.key === "Escape") setOpen(false);
        }}
      />

      {open && matches.length > 0 ? (
        <ul className="sf-autocomplete-list">
          {matches.map((option) => (
            <li key={option}>
              <button
                type="button"
                className={`sf-autocomplete-option ${option === value ? "is-current" : ""}`.trim()}
                onMouseDown={() => clearTimeout(blurTimer.current)}
                onClick={() => {
                  onChange(option);
                  setOpen(false);
                }}
              >
                {option}
              </button>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}
