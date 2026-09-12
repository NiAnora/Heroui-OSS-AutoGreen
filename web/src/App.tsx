import { useCallback, useEffect, useState } from "react";
import { Spinner, Toast } from "@heroui/react";
import { api, setUnauthorizedHandler } from "./api";
import Sidebar from "./components/Sidebar";
import Dashboard from "./pages/Dashboard";
import Login from "./pages/Login";
import Repos from "./pages/Repos";
import Settings from "./pages/Settings";

export type Page = "dashboard" | "repos" | "settings";

type AuthState = "loading" | "signed-out" | "signed-in";

export default function App() {
  const [authState, setAuthState] = useState<AuthState>("loading");
  const [page, setPage] = useState<Page>("dashboard");

  const checkSession = useCallback(async () => {
    try {
      const { authenticated } = await api.session();
      setAuthState(authenticated ? "signed-in" : "signed-out");
    } catch {
      setAuthState("signed-out");
    }
  }, []);

  useEffect(() => {
    void checkSession();
  }, [checkSession]);

  // 会话过期导致任意接口返回 401 时，退回登录页。
  useEffect(() => {
    setUnauthorizedHandler(() => setAuthState("signed-out"));
    return () => setUnauthorizedHandler(null);
  }, []);

  if (authState === "loading") {
    return (
      <div className="flex h-screen items-center justify-center bg-background">
        <Spinner size="lg" />
      </div>
    );
  }

  if (authState === "signed-out") {
    return (
      <>
        <Login
          onSuccess={() => {
            setPage("dashboard");
            setAuthState("signed-in");
          }}
        />
        <Toast.Provider placement="bottom end" />
      </>
    );
  }

  return (
    <div className="flex min-h-screen bg-background text-foreground">
      <Sidebar active={page} onChange={setPage} />
      <main className="min-w-0 flex-1">
        {page === "dashboard" && <Dashboard />}
        {page === "repos" && <Repos />}
        {page === "settings" && <Settings />}
      </main>
      <Toast.Provider placement="bottom end" />
    </div>
  );
}
