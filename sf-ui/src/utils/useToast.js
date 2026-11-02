import { useCallback } from "react";
import { toast } from "react-toastify";

import toastOptions from "./toastOptions";
import { useI18n } from "../i18n/I18nContext";

/**
 * Failures as toasts, translated.
 *
 * A toast rather than a line under the control for anything that happened
 * *away* from the form: "your AI provider refused the request" is about the
 * provider, not about the field the member last touched, and a sentence that
 * appears three screens down inside a collapsed session panel is one nobody
 * reads. Validation of a field stays beside the field.
 *
 * The message is the API's i18n code looked up in this app's dictionaries,
 * falling back to the server's English (see translateError), so a code this
 * build does not know still says something true.
 */
export default function useToastError() {
  const { tError } = useI18n();

  return useCallback(
    (error) => {
      toast.error(tError(error), toastOptions);
      return error;
    },
    [tError]
  );
}

/** The other half: a confirmation, for an action with no visible result. */
export function useToastSuccess() {
  const { t } = useI18n();

  return useCallback((key, vars) => toast.success(t(key, vars), toastOptions), [t]);
}
