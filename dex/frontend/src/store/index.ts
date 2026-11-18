import { configureStore, createSlice, type PayloadAction } from "@reduxjs/toolkit";
import type { User } from "@/api/client";

type Session = { token: string; user: User | null; lang: "zh" | "en" };

const session = createSlice({
  name: "session",
  initialState: { token: localStorage.getItem("tf_token") || "", user: null, lang: "zh" } as Session,
  reducers: {
    setAuth(state, a: PayloadAction<{ token: string; user: User }>) {
      state.token = a.payload.token;
      state.user = a.payload.user;
    },
    setUser(state, a: PayloadAction<User>) { state.user = a.payload; },
    logout(state) { state.token = ""; state.user = null; localStorage.removeItem("tf_token"); },
    setLang(state, a: PayloadAction<"zh" | "en">) { state.lang = a.payload; },
  },
});

export const { setAuth, setUser, logout, setLang } = session.actions;
export const store = configureStore({ reducer: { session: session.reducer } });
export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;
