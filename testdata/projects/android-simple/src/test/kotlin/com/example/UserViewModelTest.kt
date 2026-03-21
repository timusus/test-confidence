package com.example

import org.junit.Test
import org.junit.Ignore
import io.mockk.MockK
import io.mockk.every
import io.mockk.verify
import io.mockk.mockk

class UserViewModelTest {
    @MockK
    lateinit var repo: UserRepository

    @Test
    fun `loads users`() {
        every { repo.getUsers() } returns listOf(testUser)
        val result = viewModel.loadUsers()
        assertThat(result).hasSize(1)
    }

    @Test
    fun `saves user with verify`() {
        viewModel.saveUser(testUser)
        verify { repo.save(testUser) }
    }

    @Test
    @Ignore
    fun `ignored test`() {
    }

    @Test
    fun `test with sleep`() {
        Thread.sleep(500)
        assertNotNull(result)
    }
}
