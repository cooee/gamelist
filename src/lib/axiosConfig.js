import axios from "axios";

async function createIgdbAxiosInstance() {
  return axios.create({
    // baseURL: "https://openplatform.gateapi.io/oauth/authorize",
    headers: {
      "Content-Type": "code",
      "User-Agent": `gateio/android`,
    },
  });
}

function createLocalAxiosInstance() {
  const localAxios = axios.create();

  process.env.NODE_ENV === "development"
    ? (localAxios.defaults.baseURL =
        process.env.NEXT_PUBLIC_BASE_URL_LOCAL + "/api")
    : (localAxios.defaults.baseURL =
        process.env.NEXT_PUBLIC_DOMAIN_PROD + "/api");

  return localAxios;
}

export { createIgdbAxiosInstance, createLocalAxiosInstance };
