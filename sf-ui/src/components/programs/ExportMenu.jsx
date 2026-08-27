import { FiDownload, FiFileText, FiGrid } from "react-icons/fi";

import MenuDropdown, { MenuItem } from "../common/MenuDropdown";
import Tooltip from "../common/Tooltip";
import { useI18n } from "../../i18n/I18nContext";

/**
 * "Export" opening the two formats the API renders.
 *
 * A menu rather than two buttons: a program is exported rarely and in one of
 * two ways, and a toolbar that spends two slots on it crowds out the actions
 * people use every session. The same shape ~/cwclock's reports use.
 *
 * The trigger is the icon alone, and what it exports - the block, a week, one
 * session - is in the tooltip. Three of these sit on one screen: spelled out,
 * they crowded the headings they belong to and each one read as a different
 * kind of control, which is not what they are. The phone has drawn it this way
 * all along.
 *
 * The two formats are not the same document by accident: the PDF is what a
 * coach prints and takes to the gym, the XLSX is what an athlete fills in on a
 * laptop and mails back.
 */
export default function ExportMenu({ onExport, busy = false, label, size = "" }) {
  const { t } = useI18n();
  const hint = busy ? t("programs.exporting") : label || t("programs.export");

  return (
    <Tooltip label={hint}>
      <MenuDropdown
        trigger={busy ? <span className="sf-button-spinner" aria-hidden="true" /> : <FiDownload />}
        disabled={busy}
        triggerClassName={`sf-button sf-button-secondary sf-button-icon ${size}`.trim()}
        ariaLabel={hint}
      >
        {(close) => (
          <>
            <MenuItem
              onClick={() => {
                onExport("pdf");
                close();
              }}
            >
              <FiFileText /> {t("programs.exportPdf")}
            </MenuItem>
            <MenuItem
              onClick={() => {
                onExport("xlsx");
                close();
              }}
            >
              <FiGrid /> {t("programs.exportXlsx")}
            </MenuItem>
          </>
        )}
      </MenuDropdown>
    </Tooltip>
  );
}
