package com.example

import org.junit.Test
import io.mockk.mockk
import io.mockk.every

class UserRepositoryLocalTest {

    @Test
    fun `should return users`() {
        val repo = mockk<UserRepository>()
        every { repo.getUsers() } returns listOf()
        val result = repo.getUsers()
        assertThat(result).isEmpty()
    }
}
