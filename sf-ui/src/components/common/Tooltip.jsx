/**
 * A CSS-driven tooltip, ported from cwclock's.
 *
 * It replaces bare `title` attributes, whose native tooltip is slow to appear
 * and can't be styled - most noticeable on the collapsed sidebar, where the
 * icon is all there is to go on. With no label it renders its child untouched,
 * so a caller can switch it off (`label={collapsed ? text : null}`) without
 * branching on the whole element.
 */
export default function Tooltip({ label, position = "top", className = "", children }) {
  if (!label) return children;

  return (
    <span className={`sf-tooltip ${className}`}>
      {children}
      <span className="sf-tooltip-bubble" data-position={position} role="tooltip">
        {label}
      </span>
    </span>
  );
}
