/**
 * A toggle-styled checkbox, for settings that read as on/off rather than as
 * one of a set of choices.
 *
 * It is a real `<input type="checkbox">` underneath, invisible on top of the
 * track: the switch is only paint, so keyboard focus, the space bar, form
 * semantics and screen readers all keep working without being reimplemented.
 *
 * Ported from ~/cwclock's Switch.
 */
export default function Switch({ checked, onChange, disabled, id, ...props }) {
  return (
    <span className="sf-switch">
      <input
        id={id}
        type="checkbox"
        className="sf-switch-input"
        checked={checked}
        onChange={onChange}
        disabled={disabled}
        {...props}
      />
      <span className="sf-switch-thumb" />
    </span>
  );
}
