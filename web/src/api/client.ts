import axios from 'axios'

const client = axios.create({
  baseURL: '/api',
  headers: {
    'Content-Type': 'application/json',
    // Demo: no credentials needed — backend uses default restaurant
  },
})

export default client
