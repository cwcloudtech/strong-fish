import { Navigate, Route, Routes } from "react-router-dom";
import { ToastContainer } from "react-toastify";
import "react-toastify/dist/ReactToastify.css";

import DashboardLayout from "./components/layout/DashboardLayout";
import CookieBanner from "./components/common/CookieBanner";
import RequireAuth from "./components/layout/RequireAuth";
import { AuthProvider } from "./context/AuthContext";
import { I18nProvider } from "./i18n/I18nContext";
import { ThemeProvider, useTheme } from "./context/ThemeContext";

import About from "./pages/About";
import Admin from "./pages/dashboard/Admin";
import ApiKeys from "./pages/dashboard/ApiKeys";
import ClubDetail from "./pages/dashboard/ClubDetail";
import Contact from "./pages/Contact";
import Clubs from "./pages/dashboard/Clubs";
import Exercises from "./pages/dashboard/Exercises";
import Feed from "./pages/dashboard/Feed";
import ForgotPassword from "./pages/ForgotPassword";
import Login from "./pages/Login";
import OidcCallback from "./pages/OidcCallback";
import OneRms from "./pages/dashboard/OneRms";
import ProgramDetail from "./pages/dashboard/ProgramDetail";
import PublicProgram from "./pages/PublicProgram";
import Profile from "./pages/Profile";
import ResetPassword from "./pages/ResetPassword";
import Settings from "./pages/dashboard/Settings";
import SignUp from "./pages/SignUp";
import Training from "./pages/dashboard/Training";
import TrainingSession from "./pages/dashboard/TrainingSession";

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

  return (
    <AuthProvider>
          <Routes>
            <Route path="/" element={<Navigate to="/dashboard/feed" replace />} />
            <Route path="/login" element={<Login />} />
            <Route path="/signup" element={<SignUp />} />
            <Route path="/forgot-password" element={<ForgotPassword />} />
            <Route path="/reset-password" element={<ResetPassword />} />
            <Route path="/oidc/callback" element={<OidcCallback />} />

            {/* Public profiles are readable without a session, which is what
                makes a shared profile link work. */}
            <Route path="/profile/:handle" element={<Profile />} />
            <Route path="/about" element={<About />} />
            {/* A program its coach shared: readable by anybody holding the
                link, which is the whole point of publishing one. */}
            <Route path="/programs/:programId" element={<PublicProgram />} />
            <Route path="/contact" element={<Contact />} />

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

          <ToastContainer position="top-right" theme={theme} />
          <CookieBanner />
    </AuthProvider>
  );
}
