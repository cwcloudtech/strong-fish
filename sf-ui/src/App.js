import { Navigate, Route, Routes } from "react-router-dom";
import { ToastContainer } from "react-toastify";
import "react-toastify/dist/ReactToastify.css";

import DashboardLayout from "./components/layout/DashboardLayout";
import SignedInShell from "./components/layout/SignedInShell";
import ErrorBoundary from "./components/common/ErrorBoundary";
import CookieBanner from "./components/common/CookieBanner";
import RequireAuth from "./components/layout/RequireAuth";
import { AuthProvider } from "./context/AuthContext";
import { I18nProvider, useI18n } from "./i18n/I18nContext";
import { ThemeProvider, useTheme } from "./context/ThemeContext";

import Strength from "./pages/Strength";
import Admin from "./pages/dashboard/Admin";
import Blocks from "./pages/dashboard/Blocks";
import ApiKeys from "./pages/dashboard/ApiKeys";
import ClubDetail from "./pages/dashboard/ClubDetail";
import Contact from "./pages/Contact";
import Clubs from "./pages/dashboard/Clubs";
import Events from "./pages/dashboard/Events";
import Exercises from "./pages/dashboard/Exercises";
import Feed from "./pages/dashboard/Feed";
import Invitations from "./pages/dashboard/Invitations";
import ForgotPassword from "./pages/ForgotPassword";
import Login from "./pages/Login";
import Messages from "./pages/dashboard/Messages";
import OidcCallback from "./pages/OidcCallback";
import OneRms from "./pages/dashboard/OneRms";
import ProgramDetail from "./pages/dashboard/ProgramDetail";
import PublicPost from "./pages/PublicPost";
import PublicProgram from "./pages/PublicProgram";
import Profile from "./pages/Profile";
import ResetPassword from "./pages/ResetPassword";
import Search from "./pages/dashboard/Search";
import Settings from "./pages/dashboard/Settings";
import SignUp from "./pages/SignUp";
import Training from "./pages/dashboard/Training";
import TrainingSession from "./pages/dashboard/TrainingSession";
import MyPrograms from "./pages/dashboard/MyPrograms";

export default function App() {
  return (
    <I18nProvider>
      <ThemeProvider>
        <AppRoutes />
      </ThemeProvider>
    </I18nProvider>
  );
}

/**
 * Split out of App so it sits inside ThemeProvider and can read the resolved
 * theme - react-toastify needs it told explicitly, it can't inherit CSS
 * variables for its own backdrop.
 */
function AppRoutes() {
  const { theme } = useTheme();
  const { t } = useI18n();

  return (
    <AuthProvider>
          {/* The outermost boundary. The one inside DashboardLayout keeps the
              sidebar standing when a signed-in screen throws; this one is what
              catches everything else - a logged-out visitor on a shared link,
              and the window before the session has resolved, neither of which
              has a shell around them yet. Without it those crash to a white
              page with nothing to click. */}
          <ErrorBoundary
            title={t("errors.screenTitle")}
            message={t("errors.screenBody")}
            retryLabel={t("common.retry")}
          >
          <Routes>
            <Route path="/" element={<Navigate to="/dashboard/feed" replace />} />
            <Route path="/login" element={<Login />} />
            <Route path="/signup" element={<SignUp />} />
            <Route path="/forgot-password" element={<ForgotPassword />} />
            <Route path="/reset-password" element={<ResetPassword />} />
            <Route path="/oidc/callback" element={<OidcCallback />} />

            {/* Readable with or without a session - a shared link has to work
                for somebody with no account. SignedInShell keeps the sidebar
                around them for a member, who did not ask to leave the app just
                by opening a profile from their own feed. */}
            <Route
              path="/profile/:handle"
              element={
                <SignedInShell>
                  <Profile />
                </SignedInShell>
              }
            />
            <Route
              path="/programs/:programId"
              element={
                <SignedInShell>
                  <PublicProgram />
                </SignedInShell>
              }
            />
            <Route
              path="/posts/:postId"
              element={
                <SignedInShell>
                  <PublicPost />
                </SignedInShell>
              }
            />
            <Route
              path="/contact"
              element={
                <SignedInShell>
                  <Contact />
                </SignedInShell>
              }
            />
            {/* The calendar, readable with no account: a meet anybody can
                enter is worth finding before signing up.
                /calendar rather than /events because this is the URL people
                paste into a club chat - short, and it says what it opens. The
                same page as /dashboard/events: what a visitor gets is decided
                by the API, which returns only public events to an anonymous
                caller, and by the controls here, which all ask whether there
                is a session. */}
            {/* The powerlifting calculator, readable with no account: working
                out what a total is worth is the question somebody asks before
                they have one, and it is the page most likely to give them one.
                The form fills itself in for a member who is signed in. */}
            <Route
              path="/strength"
              element={
                <SignedInShell>
                  <Strength />
                </SignedInShell>
              }
            />
            <Route
              path="/calendar"
              element={
                <SignedInShell>
                  <Events />
                </SignedInShell>
              }
            />

            <Route
              path="/dashboard"
              element={
                <RequireAuth>
                  <DashboardLayout />
                </RequireAuth>
              }
            >
              <Route index element={<Navigate to="/dashboard/feed" replace />} />
              <Route path="feed" element={<Feed />} />
              <Route path="training" element={<Training />} />
              <Route path="training/:assignmentId" element={<TrainingSession />} />
              <Route path="one-rms" element={<OneRms />} />
              <Route path="clubs" element={<Clubs />} />
              <Route path="clubs/:clubId" element={<ClubDetail />} />
              <Route path="clubs/:clubId/programs/:programId" element={<ProgramDetail />} />
              {/* The same editor, without a club: a program somebody wrote for
                  themselves. */}
              <Route path="programs" element={<MyPrograms />} />
              <Route path="programs/:programId" element={<ProgramDetail />} />
              <Route path="events" element={<Events />} />
              <Route path="invitations" element={<Invitations />} />
              <Route path="messages" element={<Messages />} />
              <Route path="blocks" element={<Blocks />} />
              <Route path="search" element={<Search />} />
              <Route path="exercises" element={<Exercises />} />
              <Route path="settings" element={<Settings />} />
              <Route path="api-keys" element={<ApiKeys />} />
              <Route
                path="admin"
                element={
                  <RequireAuth superadmin>
                    <Admin />
                  </RequireAuth>
                }
              />
            </Route>

            <Route path="*" element={<Navigate to="/dashboard/feed" replace />} />
          </Routes>
          </ErrorBoundary>

          <ToastContainer position="top-right" theme={theme} />
          <CookieBanner />
    </AuthProvider>
  );
}
