import { FiDownload, FiFileText, FiGrid } from "react-icons/fi";

import MenuDropdown, { MenuItem } from "../common/MenuDropdown";
import { useI18n } from "../../i18n/I18nContext";

/**
 * "Export" opening the two formats the API renders.
 *
 * A menu rather than two buttons: a program is exported rarely and in one of
 * two ways, and a toolbar that spends two slots on it crowds out the actions
 * people use every session. The same shape ~/cwclock's reports use.
 *
 * The two formats are not the same document by accident: the PDF is what a
 * coach prints and takes to the gym, the XLSX is what an athlete fills in on a
 * laptop and mails back.
 */
export default function ExportMenu({ onExport, busy = false, label, size = "" }) {
  const { t } = useI18n();
  const trigger = (
    <>
      <FiDownload /> {busy ? t("programs.exporting") : label || t("programs.export")}
    </>
  );

  return (
    <MenuDropdown
      trigger={trigger}
      disabled={busy}
      triggerClassName={`sf-button sf-button-secondary ${size}`.trim()}
      title={label || t("programs.export")}
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
  );
}
