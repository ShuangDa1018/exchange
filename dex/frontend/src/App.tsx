import { useEffect, useState } from "react";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { useDispatch, useSelector } from "react-redux";
import { Layout } from "@/components/Layout";
import Trade from "@/pages/Trade";
import Assets from "@/pages/Assets";
import History from "@/pages/History";
import Rewards from "@/pages/Rewards";
import Referral from "@/pages/Referral";
import Login from "@/pages/Login";
import NotFound from "@/pages/NotFound";
import Maintenance from "@/pages/Maintenance";
import { api, getToken } from "@/api/client";
import { setUser } from "@/store";
import type { RootState } from "@/store";
import { dexSocket } from "@/api/ws";
import { datafeed } from "@/lib/datafeed";

function Guard({ children }: { children: JSX.Element }) {
  const token = useSelector((s: RootState) => s.session.token);
  if (!token) return <Navigate to="/login" replace />;
  return children;
}

export default function App() {
  const disp = useDispatch();
  const token = useSelector((s: RootState) => s.session.token);
  const [maint, setMaint] = useState(false);

  useEffect(() => {
    api.status().then((s) => {
      setMaint(s.maintenance);
      localStorage.setItem("tf_ver", s.version);
    }).catch(() => {});
    const t = window.setInterval(async () => {
      try {
        const s = await api.status();
        setMaint(s.maintenance);
        const old = localStorage.getItem("tf_ver");
        if (old && s.version && old !== s.version) location.reload();
      } catch { /* offline */ }
    }, 20000);
    return () => window.clearInterval(t);
  }, []);

  useEffect(() => {
    const tok = token || getToken();
    if (!tok) return;
    api.me().then((u) => disp(setUser(u))).catch(() => {});
    dexSocket.connect(tok);
    return () => {
      dexSocket.close();
      datafeed.clearSubs();
    };
  }, [token, disp]);

  if (maint) return <Maintenance />;

  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route element={<Layout />}>
          <Route path="/" element={<Trade />} />
          <Route path="/assets" element={<Guard><Assets /></Guard>} />
          <Route path="/history" element={<Guard><History /></Guard>} />
          <Route path="/rewards" element={<Guard><Rewards /></Guard>} />
          <Route path="/referral" element={<Guard><Referral /></Guard>} />
          <Route path="*" element={<NotFound />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}
