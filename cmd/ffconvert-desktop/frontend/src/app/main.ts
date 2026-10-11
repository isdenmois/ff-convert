import { createPinia } from "pinia";
import { createApp } from "vue";
import { MainPage } from "@/pages/main";
import "./app.css";

const app = createApp(MainPage);
app.use(createPinia());
app.mount("#app");
