import DashboardLayout from "./DashboardLayout";
import { useAuth } from "../../context/AuthContext";

/**
 * Wraps a page that is readable both ways: with the app's chrome for a member,
 * and bare for a visitor.
 *
 * A profile, a shared post, a published program and the contact form all have
 * to work for somebody with no account - that is what makes a shared link a
 * link. But a member who clicks through to one of them from their own feed was
 * not asking to leave the app, and dropping them onto a page with no navigation
 * leaves them with the browser's back button as the only way home.
 *
 * So the page is the same either way; only the frame around it differs.
 */
export default function SignedInShell({ children }) {
  const { user } = useAuth();
  return user ? <DashboardLayout>{children}</DashboardLayout> : children;
}
