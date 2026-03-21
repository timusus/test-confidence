package com.example

import org.junit.Test
import io.mockk.every
import io.mockk.mockk

class UserRepositoryTest {
    private val api = mockk<ApiClient>()
    private val repo = UserRepository(api)

    @Test
    fun `tautological test`() {
        val user = User("test")
        every { api.getUser() } returns user
        val result = repo.getUser()
        assertThat(result).isEqualTo(user)
    }

    @Test
    fun `proper behavioral test`() {
        every { api.getUser() } returns User("raw")
        val result = repo.getUserName()
        assertEquals("raw", result)
    }
}
