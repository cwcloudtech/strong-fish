import { useEffect, useRef, useState } from "react";
import { FiCheck, FiChevronDown } from "react-icons/fi";

/**
 * A pill-shaped dropdown, ported from ~/uprodit's LanguageDropdown: a compact
 * toggle showing an icon, a short code and a caret, opening a popup list where
 * each option carries a code chip, its full label and a check on the current
 * one.
 *
 * It's a real listbox rather than a styled <select> because a native one can't
 * be given that layout - and because the sidebar's two dropdowns sit on the
 * navy chrome, where a browser's own select styling looks out of place.
 *
 * `variant` picks the colour scheme: "dark" for the navy chrome, "light" for a
 * normal surface.
 */
export default function Dropdown({
  icon,
  value,
  options,
  onChange,
  variant = "light",
  ariaLabel,
  align = "right",
}) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef(null);

  useEffect(() => {
    if (!open) return;

    const onPointerDown = (event) => {
      if (rootRef.current && !rootRef.current.contains(event.target)) setOpen(false);
    };
    const onKeyDown = (event) => {
      if (event.key === "Escape") setOpen(false);
    };

    document.addEventListener("mousedown", onPointerDown);
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("mousedown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [open]);

  const current = options.find((option) => option.value === value) || options[0];

  return (
    <div className={`sf-dropdown sf-dropdown-${variant}`} ref={rootRef}>
      <button
        type="button"
        className="sf-dropdown-toggle"
        onClick={() => setOpen((current_) => !current_)}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-label={ariaLabel}
      >
        {icon ? <span className="sf-dropdown-icon">{icon}</span> : null}
        <span>{current?.code ?? current?.label}</span>
        <FiChevronDown className={`sf-dropdown-caret ${open ? "is-open" : ""}`} aria-hidden="true" />
      </button>

      {open ? (
        <ul className={`sf-dropdown-menu sf-dropdown-menu-${align}`} role="listbox">
          {options.map((option) => (
            <li key={option.value} role="option" aria-selected={option.value === value}>
              <button
                type="button"
                className={`sf-dropdown-item ${option.value === value ? "active" : ""}`}
                onClick={() => {
                  onChange(option.value);
                  setOpen(false);
                }}
              >
                {option.code ? <span className="sf-dropdown-item-code">{option.code}</span> : null}
                <span className="sf-dropdown-item-label">{option.label}</span>
                {option.value === value ? <FiCheck className="sf-dropdown-check" aria-hidden="true" /> : null}
              </button>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}
