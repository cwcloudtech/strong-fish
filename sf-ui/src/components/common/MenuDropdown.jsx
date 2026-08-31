import { useEffect, useRef, useState } from "react";

/**
 * An action menu: a trigger button that opens a small list of things to do,
 * closed on an outside click or Escape.
 *
 * Ported from ~/cwclock's Dropdown, which its reports use for exactly this -
 * "Export" opening "CSV / PDF". Named apart from this app's own Dropdown,
 * which is a different thing entirely: that one picks a *value* and shows
 * which is current (the language, the theme), while this one runs a command
 * and has no state to display.
 *
 * Children may be a render prop taking `close`, so an item can do its work and
 * dismiss the menu without every caller wiring that up.
 */
export default function MenuDropdown({
  trigger,
  align = "end",
  className = "",
  triggerClassName = "sf-button sf-button-secondary",
  title,
  // What the trigger is called when it carries no text of its own - an
  // icon-only button is otherwise announced as "button" and nothing more.
  ariaLabel,
  disabled,
  children,
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

  return (
    <div className={`sf-menu ${className}`.trim()} ref={rootRef}>
      <button
        type="button"
        className={triggerClassName}
        onClick={() => setOpen((current) => !current)}
        title={title}
        aria-label={ariaLabel}
        disabled={disabled}
        aria-haspopup="menu"
        aria-expanded={open}
      >
        {trigger}
      </button>
      {!disabled && open ? (
        <div className={`sf-menu-list ${align === "end" ? "align-end" : ""}`} role="menu">
          {typeof children === "function" ? children(() => setOpen(false)) : children}
        </div>
      ) : null}
    </div>
  );
}

/**
 * One command in the menu.
 *
 * A button by default, because that is what a command is. `as` takes anything
 * else that renders - a router Link, for the menus whose items are navigation
 * rather than actions - so those keep working as links: middle-click, open in
 * a new tab, and a status bar showing where they go.
 */
export function MenuItem({ as: Component = "button", className = "", ...props }) {
  const type = Component === "button" ? { type: "button" } : {};
  return <Component role="menuitem" className={`sf-menu-item ${className}`.trim()} {...type} {...props} />;
}

/** A heading or a note between items - not clickable. */
export function MenuText({ className = "", ...props }) {
  return <div className={`sf-menu-text ${className}`.trim()} {...props} />;
}

export function MenuDivider() {
  return <div className="sf-menu-divider" />;
}
