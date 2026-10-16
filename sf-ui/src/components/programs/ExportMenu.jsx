import { FiDownload, FiFileText, FiGrid, FiImage } from "react-icons/fi";

import MenuDropdown, { MenuItem } from "../common/MenuDropdown";
import Tooltip from "../common/Tooltip";
import { useI18n } from "../../i18n/I18nContext";

/**
 * "Export" opening the formats the API renders.
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
 * The formats are not the same document by accident: the PDF is what a coach
 * prints and takes to the gym, the XLSX is what an athlete fills in on a
 * laptop and mails back, and the image is what either of them pastes into a
 * conversation - one picture, no attachment to open.
 *
 * formats narrows the list for a screen that does not have all three - health
 * data is a picture or a printout, never a spreadsheet to fill in.
 */
const ALL_FORMATS = ["pdf", "xlsx", "png"];

export default function ExportMenu({ onExport, busy = false, label, size = "", formats = ALL_FORMATS }) {
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
            {formats.includes("pdf") ? (
              <MenuItem
                onClick={() => {
                  onExport("pdf");
                  close();
                }}
              >
                <FiFileText /> {t("programs.exportPdf")}
              </MenuItem>
            ) : null}
            {formats.includes("xlsx") ? (
              <MenuItem
                onClick={() => {
                  onExport("xlsx");
                  close();
                }}
              >
                <FiGrid /> {t("programs.exportXlsx")}
              </MenuItem>
            ) : null}
            {formats.includes("png") ? (
              <MenuItem
                onClick={() => {
                  onExport("png");
                  close();
                }}
              >
                <FiImage /> {t("programs.exportPng")}
              </MenuItem>
            ) : null}
          </>
        )}
      </MenuDropdown>
    </Tooltip>
  );
}
