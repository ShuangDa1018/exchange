import i18n from "i18next";
import { initReactI18next } from "react-i18next";

const resources = {
  zh: {
    translation: {
      trade: "交易",
      assets: "资产",
      rewards: "积分",
      referral: "推荐",
      history: "记录",
      login: "登录",
      register: "注册",
      logout: "退出",
      long: "开多",
      short: "开空",
      close: "平仓",
      deposit: "充值",
      withdraw: "提现",
      available: "可用",
      margin: "保证金",
      liq: "强平价",
      pnl: "未实现盈亏",
      leverage: "杠杆",
      qty: "数量",
      price: "价格",
      mark: "标记价格",
      maintenance: "系统维护中",
      offline: "网络已断开，正在重连…",
      slow: "当前网络较慢",
      crash: "页面崩溃了",
      notfound: "页面不存在",
    },
  },
  en: {
    translation: {
      trade: "Trade",
      assets: "Assets",
      rewards: "Points",
      referral: "Referral",
      history: "History",
      login: "Login",
      register: "Register",
      logout: "Logout",
      long: "Long",
      short: "Short",
      close: "Close",
      deposit: "Deposit",
      withdraw: "Withdraw",
      available: "Available",
      margin: "Margin",
      liq: "Liq. Price",
      pnl: "uPnL",
      leverage: "Leverage",
      qty: "Qty",
      price: "Price",
      mark: "Mark",
      maintenance: "Under maintenance",
      offline: "Offline, reconnecting…",
      slow: "Slow network",
      crash: "Something crashed",
      notfound: "Not found",
    },
  },
};

i18n.use(initReactI18next).init({
  resources,
  lng: "zh",
  fallbackLng: "en",
  interpolation: { escapeValue: false },
});

export default i18n;
