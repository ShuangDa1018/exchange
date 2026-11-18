import { Box, Button, Flex, HStack, Text, useBreakpointValue } from "@chakra-ui/react";
import { Link as RLink, Outlet, useLocation, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useDispatch, useSelector } from "react-redux";
import type { RootState } from "@/store";
import { logout, setLang } from "@/store";
import { clearToken } from "@/api/client";
import { dexSocket } from "@/api/ws";
import { useEffect, useState } from "react";
import i18n from "@/i18n";

export function Layout() {
  const { t } = useTranslation();
  const loc = useLocation();
  const nav = useNavigate();
  const disp = useDispatch();
  const user = useSelector((s: RootState) => s.session.user);
  const compact = useBreakpointValue({ base: true, md: false });
  const [online, setOnline] = useState(navigator.onLine);
  const [slow, setSlow] = useState(false);

  useEffect(() => {
    const on = () => setOnline(true);
    const off = () => setOnline(false);
    window.addEventListener("online", on);
    window.addEventListener("offline", off);
    const conn = (navigator as any).connection;
    const check = () => setSlow(conn && (conn.effectiveType === "2g" || conn.effectiveType === "slow-2g"));
    conn?.addEventListener?.("change", check);
    check();
    return () => {
      window.removeEventListener("online", on);
      window.removeEventListener("offline", off);
      conn?.removeEventListener?.("change", check);
    };
  }, []);

  const tabs = [
    { to: "/", label: t("trade") },
    { to: "/assets", label: t("assets") },
    { to: "/history", label: t("history") },
    { to: "/rewards", label: t("rewards") },
    { to: "/referral", label: t("referral") },
  ];

  return (
    <Flex direction="column" minH="100vh" bg="bg.900">
      {!online && <Box bg="orange.500" color="black" textAlign="center" py={1} fontSize="sm">{t("offline")}</Box>}
      {slow && online && <Box bg="whiteAlpha.200" textAlign="center" py={1} fontSize="xs" color="gray.300">{t("slow")}</Box>}
      <Flex px={{ base: 3, md: 5 }} py={3} align="center" borderBottomWidth="1px" borderColor="whiteAlpha.100" gap={4}>
        <Text fontWeight="800" letterSpacing="-0.04em" fontSize="lg" cursor="pointer" onClick={() => nav("/")}>TF</Text>
        <HStack spacing={1} overflowX="auto">
          {tabs.map((x) => (
            <Button key={x.to} as={RLink} to={x.to} variant={loc.pathname === x.to ? "solid" : "ghost"} size="sm">
              {x.label}
            </Button>
          ))}
        </HStack>
        <Box flex="1" />
        <Button size="xs" variant="ghost" onClick={() => { const l = i18n.language === "zh" ? "en" : "zh"; i18n.changeLanguage(l); disp(setLang(l)); }}>
          {i18n.language === "zh" ? "EN" : "中"}
        </Button>
        {user ? (
          <HStack>
            {!compact && <Text fontSize="sm" color="gray.400">{user.display_name}</Text>}
            <Button variant="outline" onClick={() => { clearToken(); dexSocket.close(); disp(logout()); nav("/login"); }}>{t("logout")}</Button>
          </HStack>
        ) : (
          <Button colorScheme="teal" onClick={() => nav("/login")}>{t("login")}</Button>
        )}
      </Flex>
      <Box flex="1" px={{ base: 2, md: 4 }} py={3}>
        <Outlet />
      </Box>
    </Flex>
  );
}
