import { FiCpu } from "react-icons/fi";

import MenuDropdown, { MenuItem, MenuText } from "../common/MenuDropdown";
import Tooltip from "../common/Tooltip";
import { useI18n } from "../../i18n/I18nContext";

/**
 * The button that asks the model, and - when the member has more than one
 * subscription - which one to spend.
 *
 * One connection is the common case and must stay one click: somebody with a
 * single key should never be made to pick it. Two or more is a coach's key
 * beside the member's own, and then *which one pays* is a real question, so
 * the button becomes a menu of them. The API applies the same rule from the
 * other side: no connection named means the member's default.
 */
export default function AskButton({
  connections = [],
  onAsk,
  busy = false,
  disabled = false,
  label,
  tooltip,
  size = "sf-button-sm",
}) {
  const { t } = useI18n();
  const icon = busy ? <span className="sf-button-spinner" aria-hidden="true" /> : <FiCpu />;

  if (connections.length > 1) {
    return (
      <MenuDropdown
        trigger={
          <>
            {icon} {label}
          </>
        }
        align="start"
        triggerClassName={`sf-button sf-button-secondary ${size}`.trim()}
        disabled={disabled || busy}
        title={tooltip}
        ariaLabel={label}
      >
        {(close) => (
          <>
            <MenuText>{t("ai.askWith")}</MenuText>
            {connections.map((connection) => (
              <MenuItem
                key={connection.id}
                onClick={() => {
                  onAsk(connection.id);
                  close();
                }}
              >
                {connectionLabel(connection, t)}
              </MenuItem>
            ))}
          </>
        )}
      </MenuDropdown>
    );
  }

  return (
    <Tooltip label={tooltip}>
      <button
        type="button"
        className={`sf-button sf-button-secondary ${size}`.trim()}
        onClick={() => onAsk(connections[0]?.id || "")}
        disabled={disabled || busy}
      >
        {icon} {label}
      </button>
    </Tooltip>
  );
}

/**
 * One connection in a line: what the member called it, or its provider, and
 * whose it is when it is not theirs - "Claude (Marie)" is the only way to
 * tell a lent key from your own in a menu.
 */
export function connectionLabel(connection, t) {
  const name = connection.name || connection.providerLabel || connection.provider;
  if (connection.role && connection.role !== "owner" && connection.ownerName) {
    return `${name} (${connection.ownerName})`;
  }
  return name;
}
