import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { FiChevronDown } from "react-icons/fi";

/**
 * The shell both selects are built on: a trigger that looks like an input, and
 * a panel that opens under it with a search box in it.
 *
 * Ported from ~/cwclock's MultiSelect/AutocompleteSelect, which share one
 * trigger and one panel and differ only in what the list does when you click a
 * row. Keeping that split here means the two cannot drift apart visually.
 *
 * The panel is portaled to document.body and placed from the trigger's own
 * bounding rect, for the reason the Tooltip is: these sit inside modals and
 * inside cards that scroll, and a scroll container clips its overflow in *both*
 * axes whatever overflow-x says. A CSS-anchored panel hanging below a field
 * near the bottom of a modal is simply cut off.
 */

/** The gap between the trigger and the panel. */
const OFFSET = 4;

/** Roughly the panel's height, used to decide whether it opens upward. */
const ESTIMATED_HEIGHT = 300;

export default function SelectPanel({
  label,
  summary,
  placeholder,
  disabled,
  searchPlaceholder,
  query,
  onQueryChange,
  id,
  className = "",
  children,
}) {
  const triggerRef = useRef(null);
  const panelRef = useRef(null);
  const [origin, setOrigin] = useState(null);
  const open = origin !== null;

  const close = useCallback(() => {
    setOrigin(null);
    onQueryChange?.("");
  }, [onQueryChange]);

  const place = useCallback(() => {
    const rect = triggerRef.current?.getBoundingClientRect();
    if (rect) setOrigin(rect);
  }, []);

  // Closing rather than following: the panel is positioned in viewport
  // coordinates, and anything that moves the trigger invalidates them.
  useEffect(() => {
    if (!open) return undefined;

    const onPointerDown = (event) => {
      if (triggerRef.current?.contains(event.target)) return;
      if (panelRef.current?.contains(event.target)) return;
      close();
    };
    const onKeyDown = (event) => {
      if (event.key === "Escape") close();
    };

    document.addEventListener("mousedown", onPointerDown);
    document.addEventListener("keydown", onKeyDown);
    window.addEventListener("scroll", close, true);
    window.addEventListener("resize", close);
    return () => {
      document.removeEventListener("mousedown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown);
      window.removeEventListener("scroll", close, true);
      window.removeEventListener("resize", close);
    };
  }, [open, close]);

  // Measured before paint: a panel that flips upward after being painted below
  // is a visible jump.
  const [flipped, setFlipped] = useState(false);
  useLayoutEffect(() => {
    if (!origin) return;
    setFlipped(origin.bottom + ESTIMATED_HEIGHT > window.innerHeight && origin.top > ESTIMATED_HEIGHT);
  }, [origin]);

  return (
    <>
      <button
        id={id}
        ref={triggerRef}
        type="button"
        className={`sf-select-trigger ${open ? "is-open" : ""} ${className}`}
        onClick={() => (open ? close() : place())}
        disabled={disabled}
        aria-haspopup="listbox"
        aria-expanded={open}
      >
        {label ? <span className="sf-select-trigger-label">{label}</span> : null}
        <span className={`sf-select-trigger-summary ${summary ? "" : "is-placeholder"}`}>
          {summary || placeholder}
        </span>
        <FiChevronDown className={`sf-select-caret ${open ? "is-open" : ""}`} aria-hidden="true" />
      </button>

      {open
        ? createPortal(
            <div
              ref={panelRef}
              className="sf-select-panel"
              style={{
                left: origin.left,
                // Matched to the trigger so the panel reads as part of the
                // field rather than as a floating menu.
                minWidth: origin.width,
                ...(flipped
                  ? { bottom: window.innerHeight - origin.top + OFFSET }
                  : { top: origin.bottom + OFFSET }),
              }}
            >
              {onQueryChange ? (
                <input
                  className="sf-input sf-select-search"
                  type="text"
                  placeholder={searchPlaceholder}
                  value={query}
                  onChange={(event) => onQueryChange(event.target.value)}
                  autoFocus
                />
              ) : null}
              {children(close)}
            </div>,
            document.body
          )
        : null}
    </>
  );
}
